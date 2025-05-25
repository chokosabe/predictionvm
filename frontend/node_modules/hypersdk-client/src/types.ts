import { VMABI } from "./Marshaler"

export interface SignerIface {
    signTx(txPayload: TransactionPayload, abi: VMABI): Promise<Uint8Array>
    getPublicKey(): Uint8Array
    connect(): Promise<void>
}


export type ActionOutput = any;

export type ActionData = {
    actionName: string
    data: Record<string, unknown>
}

export type TransactionPayload = {
    base: TransactionBase,
    actions: ActionData[]
}

export type TransactionBase = {
    timestamp: string
    chainId: string
    maxFee: string
}

export type Units = {
    bandwidth: number
    compute: number
    storageRead: number
    storageAllocate: number
    storageWrite: number
}
