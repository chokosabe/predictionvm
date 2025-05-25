import { base64 } from '@scure/base';
import { APIBlock, APITxResult } from './apiTransformers';

interface ApiResponse<T> {
    result: T;
    error?: {
        message: string;
    };
}

interface NetworkInfo {
    networkId: number;
    subnetId: string;
    chainId: string;
}


export class HyperSDKHTTPClient {
    private getNetworkCache: NetworkInfo | null = null;

    constructor(
        private readonly apiHost: string,
        private readonly vmName: string,
        private readonly vmRPCPrefix: string
    ) {
        if (this.vmRPCPrefix.startsWith('/')) {
            this.vmRPCPrefix = vmRPCPrefix.substring(1);
        }
    }

    public async makeCoreAPIRequest<T>(method: string, params: object = {}): Promise<T> {
        return this.makeApiRequest("coreapi", `hypersdk.${method}`, params);
    }

    public async makeVmAPIRequest<T>(method: string, params: object = {}): Promise<T> {
        return this.makeApiRequest(this.vmRPCPrefix, `${this.vmName}.${method}`, params);
    }

    public async makeIndexerRequest<T>(method: string, params: object = {}): Promise<T> {
        return this.makeApiRequest("indexer", `indexer.${method}`, params);
    }

    private async makeApiRequest<T>(namespace: string, method: string, params: object = {}): Promise<T> {
        const controller = new AbortController();
        const TIMEOUT_SEC = 10
        const timeoutId = setTimeout(() => controller.abort(), TIMEOUT_SEC * 1000);

        try {
            const response = await fetch(`${this.apiHost}/ext/bc/${this.vmName}/${namespace}`, {
                method: "POST",
                headers: {
                    "Content-Type": "application/json"
                },
                body: JSON.stringify({
                    jsonrpc: "2.0",
                    method,
                    params,
                    id: parseInt(String(Math.random()).slice(2))
                }),
                signal: controller.signal
            });

            const json: ApiResponse<T> = await response.json();
            if (json?.error?.message) {
                throw new Error(json.error.message);
            }
            return json.result;
        } catch (error: unknown) {
            if (error instanceof Error && error.name === 'AbortError') {
                throw new Error(`Request timed out after ${TIMEOUT_SEC} seconds`);
            }
            throw error;
        } finally {
            clearTimeout(timeoutId);
        }
    }

    public async getNetwork(): Promise<NetworkInfo> {
        if (!this.getNetworkCache) {
            this.getNetworkCache = await this.makeCoreAPIRequest<NetworkInfo>('network');
        }
        return this.getNetworkCache;
    }

    public async sendRawTx(txBytes: Uint8Array): Promise<{ txId: string }> {
        const bytesBase64 = base64.encode(txBytes);
        return this.makeCoreAPIRequest<{ txId: string }>('submitTx', { tx: bytesBase64 });
    }

    public async executeActions(actions: Uint8Array[], actor: string): Promise<string[]> {
        const { outputs, error } = await this.makeCoreAPIRequest<{ outputs?: string[], error?: string }>('executeActions', {
            actions: actions.map(action => base64.encode(action)),
            actor: actor,
        });

        if (error) {
            throw new Error(error);
        } else if (outputs) {
            return outputs;
        } else {
            throw new Error("No output or error returned from execute");
        }
    }

    public async getTransactionStatus(txId: string): Promise<APITxResult> {
        return this.makeIndexerRequest<APITxResult>('getTx', { txId });
    }

    public async getBlock(blockID: string): Promise<APIBlock> {
        return this.makeIndexerRequest<APIBlock>('getBlock', { blockID });
    }

    public async getBlockByHeight(height: number): Promise<APIBlock> {
        return this.makeIndexerRequest<APIBlock>('getBlockByHeight', { height });
    }

    public async getLatestBlock(): Promise<APIBlock> {
        return this.makeIndexerRequest<APIBlock>('getLatestBlock', {});
    }
}
