import { SignerIface } from "./types";
import { ed25519 } from "@noble/curves/ed25519";
import { ED25519_AUTH_ID, Marshaler, VMABI } from "./Marshaler";
import { TransactionPayload } from "./types";

export class PrivateKeySigner implements SignerIface {
    constructor(private privateKey: Uint8Array) {
        if (this.privateKey.length !== 32) {
            throw new Error("Private key must be 32 bytes");
        }
    }

    async signTx(txPayload: TransactionPayload, abi: VMABI): Promise<Uint8Array> {
        const marshaler = new Marshaler(abi);
        const digest = marshaler.encodeTransaction(txPayload);
        const signature = ed25519.sign(digest, this.privateKey);

        const pubKey = this.getPublicKey()

        return new Uint8Array([...digest, ED25519_AUTH_ID, ...pubKey, ...signature])
    }

    getPublicKey(): Uint8Array {
        return ed25519.getPublicKey(this.privateKey);
    }

    async connect(): Promise<void> {
        // No-op
    }

    public static debugExtractPublicKey(signed: Uint8Array): Uint8Array {
        const pubKey = signed.slice(-1 * (64 + 32)).slice(0, 32)
        return pubKey
    }
}


export class EphemeralSigner extends PrivateKeySigner {
    constructor() {
        super(ed25519.utils.randomPrivateKey());
    }
}

