import { decodeNumber, encodeNumber } from "./Marshaler";

export function decodeBatchMessage(messages: Uint8Array): Uint8Array[] {
    const messageCount = decodeNumber("uint32", messages.subarray(0, 4)) as number;
    let result: Uint8Array[] = []
    let offset = 4
    for (let i = 0; i < messageCount; i++) {
        const messageLength = decodeNumber("uint32", messages.subarray(offset, offset + 4)) as number;
        result.push(messages.subarray(offset + 4, offset + 4 + messageLength))
        offset += 4 + messageLength
    }
    if (offset !== messages.length) {
        throw new Error("Invalid batch message")
    }
    return result
}

export function encodeBatchMessage(messages: Uint8Array[]): Uint8Array {
    const messageCountBytes = encodeNumber("uint32", messages.length);

    let result = new Uint8Array([...messageCountBytes])
    for (const message of messages) {
        const lenBytes = encodeNumber("uint32", message.length)
        result = new Uint8Array([...result, ...lenBytes, ...message])
    }
    return result;
}
