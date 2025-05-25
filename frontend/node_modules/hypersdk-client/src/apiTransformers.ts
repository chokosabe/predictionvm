import { hexToBytes } from "@noble/hashes/utils";
import { Marshaler } from "./Marshaler";
import { ActionOutput } from "./types";
import { Units } from "./types";
import { bytesToHex } from "@noble/hashes/utils";

export type APITxResult = {
    timestamp: number;
    success: boolean;
    units: {
        bandwidth: number;
        compute: number;
        storageRead: number;
        storageAllocate: number;
        storageWrite: number;
    };
    fee: number;
    result: string[];
}

export type TxResult = Omit<APITxResult, 'result'> & {
    result: ActionOutput[]
}

export function processAPITxResult(response: APITxResult, marshaler: Marshaler): TxResult {
    return {
        ...response,
        result: response.result.map(output =>
            marshaler.parseTyped(hexToBytes(output), "output")[0]
        )
    }
}

export type APITransactionStatus = {
    success: boolean;
    units: Units;
    fee: number;
    outputs: string[];
    error: string;
}

export type TransactionStatus = Omit<APITransactionStatus, 'outputs'> & {
    outputs: Record<string, unknown>[];
}


export function processAPITransactionStatus(response: APITransactionStatus, marshaler: Marshaler): TransactionStatus {
    const processedOutputs = response.outputs.map(output =>
        marshaler.parseTyped(hexToBytes(output), "output")[0]
    );

    return {
        ...response,
        outputs: processedOutputs,
    };
}


export function processAPIBlock(response: APIBlock, marshaler: Marshaler): Block {
    const processedTxs = response.block.block.txs.map(tx => ({
        ...tx,
        auth: {
            signer: bytesToHex(new Uint8Array(tx.auth.signer)),
            signature: bytesToHex(new Uint8Array(tx.auth.signature))
        }
    }));

    const processedResults = response.block.results.map(result =>
        processAPITransactionStatus(result, marshaler)
    );

    return {
        ...response.block,
        block: {
            ...response.block.block,
            txs: processedTxs,
        },
        results: processedResults,
    }
}

type absctractBlock<FixedBytesType, TxStatusType> = {
    blockID: string;
    block: {
        parent: string;
        timestamp: number;
        height: number;
        txs: {
            base: {
                timestamp: number;
                chainId: string;
                maxFee: number;
            };
            actions: Record<string, unknown>[];
            auth: {
                signer: FixedBytesType;
                signature: FixedBytesType;
            };
        }[];
        stateRoot: string;
    };
    results: TxStatusType[];
    unitPrices: Units;
}

export type APIBlock = {
    block: absctractBlock<number[], APITransactionStatus>,
    blockBytes: string
}
export type Block = absctractBlock<string, TransactionStatus>
