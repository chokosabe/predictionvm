import { ActionOutput, SignerIface } from './types';
import { EphemeralSigner, PrivateKeySigner } from './PrivateKeySigner';
import { DEFAULT_SNAP_ID, MetamaskSnapSigner } from './MetamaskSnapSigner';
import { addressHexFromPubKey, Marshaler, VMABI } from './Marshaler';
import { HyperSDKHTTPClient } from './HyperSDKHTTPClient';
import { base58, base64 } from '@scure/base';
import { Block, processAPIBlock, processAPITransactionStatus, processAPITxResult, TransactionStatus, TxResult } from './apiTransformers';
import { sha256 } from '@noble/hashes/sha256';
import { ActionData, TransactionPayload } from './types';

// TODO: Implement fee prediction
const DEFAULT_MAX_FEE = 1000000n;
const DECIMALS = 9;

type SignerType =
    | { type: "ephemeral" }
    | { type: "private-key", privateKey: Uint8Array }
    | { type: "metamask-snap", snapId?: string };

export class HyperSDKClient extends EventTarget {
    private readonly http: HyperSDKHTTPClient;
    private signer: SignerIface | null = null;
    private abi: VMABI | null = null;
    private marshaler: Marshaler | null = null;
    private blockSubscribers: Set<(block: Block) => void> = new Set();
    private isPollingBlocks: boolean = false;

    constructor(
        apiHost: string,
        vmName: string,
        vmRPCPrefix: string
    ) {
        super();
        this.http = new HyperSDKHTTPClient(apiHost, vmName, vmRPCPrefix);
    }

    // Public methods

    public async connectWallet(params: SignerType): Promise<SignerIface> {
        this.signer = this.createSigner(params);
        await this.signer.connect();
        this.dispatchEvent(new CustomEvent('signerConnected', { detail: this.signer }));
        return this.signer;
    }

    public async sendTransaction(actions: ActionData[]): Promise<TxResult> {
        const txPayload = await this.createTransactionPayload(actions);
        const abi = await this.getAbi();
        const signed = await this.getSigner().signTx(txPayload, abi);
        const { txId } = await this.http.sendRawTx(signed);
        return this.waitForTransaction(txId);
    }

    //actorHex is optional, if not provided, the signer's public key will be used
    public async executeActions(actions: ActionData[], actorHex?: string): Promise<ActionOutput[]> {
        const marshaler = await this.getMarshaler();
        const actionBytesArray = actions.map(action => marshaler.encodeTyped(action.actionName, JSON.stringify(action.data)));

        const actor = actorHex ?? addressHexFromPubKey(this.getSigner().getPublicKey());

        const output = await this.http.executeActions(
            actionBytesArray,
            actor
        );

        return output.map(output => marshaler.parseTyped(base64.decode(output), "output")[0]);
    }

    public async getBalance(address: string): Promise<bigint> {
        const result = await this.http.makeVmAPIRequest<{ amount: number }>('balance', { address });
        return BigInt(result.amount); // TODO: Handle potential precision loss
    }

    public convertToNativeTokens(formattedBalance: string): bigint {
        const float = parseFloat(formattedBalance);
        return BigInt(float * 10 ** DECIMALS);
    }

    public formatNativeTokens(balance: bigint): string {
        const divisor = 10n ** BigInt(DECIMALS);
        const quotient = balance / divisor;
        const remainder = balance % divisor;
        const paddedRemainder = remainder.toString().padStart(DECIMALS, '0');
        return `${quotient}.${paddedRemainder}`;
    }

    public async getAbi(): Promise<VMABI> {
        if (!this.abi) {
            const result = await this.http.makeCoreAPIRequest<{ abi: VMABI }>('getABI');
            this.abi = result.abi;
        }
        return this.abi;
    }

    public async getTransactionStatus(txId: string): Promise<TxResult> {
        const response = await this.http.getTransactionStatus(txId);
        const marshaler = await this.getMarshaler();
        return processAPITxResult(response, marshaler);
    }

    public async listenToBlocks(callback: (block: Block) => void, includeEmpty: boolean = false, pollingRateMs: number = 300): Promise<() => void> {
        this.blockSubscribers.add(callback);

        if (!this.isPollingBlocks) {
            this.startPollingBlocks(includeEmpty, pollingRateMs);
        }

        return () => {
            this.blockSubscribers.delete(callback);
        };
    }

    // Private methods

    private async startPollingBlocks(includeEmpty: boolean, pollingRateMs: number) {
        this.isPollingBlocks = true;
        const marshaler = await this.getMarshaler();
        let currentHeight: number = -1;

        const fetchNextBlock = async () => {
            if (!this.isPollingBlocks) return;

            try {
                const block = currentHeight === -1 ?
                    await this.http.getLatestBlock()
                    : await this.http.getBlockByHeight(currentHeight + 1);

                currentHeight = block.block.block.height;

                if (includeEmpty || block.block.block.txs.length > 0) {
                    const executedBlock = processAPIBlock(block, marshaler);
                    this.blockSubscribers.forEach(callback => {
                        try {
                            callback(executedBlock);
                        } catch (error) {
                            console.error("Error in block callback", error);
                        }
                    });
                }

                setTimeout(fetchNextBlock, pollingRateMs);
            } catch (error: any) {
                if (error?.message?.includes("block not found")) {
                    setTimeout(fetchNextBlock, pollingRateMs * 2);
                } else {
                    console.error(error);
                    setTimeout(fetchNextBlock, pollingRateMs * 2); // Longer delay on error
                }
            }
        };

        fetchNextBlock();
    }

    private createSigner(params: SignerType): SignerIface {
        switch (params.type) {
            case "ephemeral":
                return new EphemeralSigner();
            case "private-key":
                return new PrivateKeySigner(params.privateKey);
            case "metamask-snap":
                return new MetamaskSnapSigner(params.snapId ?? DEFAULT_SNAP_ID);
            default:
                throw new Error(`Invalid signer type: ${(params as { type: string }).type}`);
        }
    }

    private getSigner(): SignerIface {
        if (!this.signer) {
            throw new Error("Signer not connected");
        }
        return this.signer;
    }


    private async getMarshaler(): Promise<Marshaler> {
        if (!this.marshaler) {
            const abi = await this.getAbi();
            this.marshaler = new Marshaler(abi);
        }
        return this.marshaler;
    }

    private async createTransactionPayload(actions: ActionData[]): Promise<TransactionPayload> {
        const { chainId } = await this.http.getNetwork();
        const chainIdBigNumber = idStringToBigInt(chainId);

        return {
            base: {
                timestamp: String(BigInt(Date.now()) + 59n * 1000n),
                chainId: String(chainIdBigNumber),
                maxFee: String(DEFAULT_MAX_FEE),
            },
            actions: actions
        };
    }

    private async waitForTransaction(txId: string, timeout: number = 55000): Promise<TxResult> {
        const startTime = Date.now();
        let lastError: Error | null = null;
        for (let i = 0; i < 10; i++) {
            if (Date.now() - startTime > timeout) {
                throw new Error("Transaction wait timed out");
            }
            try {
                return await this.getTransactionStatus(txId);
            } catch (error) {
                lastError = error instanceof Error ? error : new Error(String(error));
                if (!(error instanceof Error) || error.message !== "tx not found") {
                    throw error;
                }
            }
            await new Promise(resolve => setTimeout(resolve, 100 * i));
        }
        throw lastError || new Error("Failed to get transaction status");
    }
}



const cb58 = {
    encode(data: Uint8Array): string {
        return base58.encode(new Uint8Array([...data, ...sha256(data).subarray(-4)]));
    },
    decode(string: string): Uint8Array {
        return base58.decode(string).subarray(0, -4);
    },
};

export function idStringToBigInt(id: string): bigint {
    const bytes = cb58.decode(id);
    return BigInt(`0x${bytes.reduce((str, byte) => str + byte.toString(16).padStart(2, '0'), '')}`);
}
