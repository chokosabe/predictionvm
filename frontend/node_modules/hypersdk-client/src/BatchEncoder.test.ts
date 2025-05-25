import { expect, test } from 'vitest'
import { decodeBatchMessage, encodeBatchMessage } from './BatchEncoder';
import { bytesToHex } from '@noble/curves/abstract/utils';

test('Encode batch message', () => {
    const expectedHex = "00000002" + //2 messages
        "00000009" + //9 bytes message length
        "010203040506070809" + //bytes of message 1
        "00000001" + //1 byte message length
        "0a" //bytes of message 2

    const actualBytes = encodeBatchMessage([
        new Uint8Array([1, 2, 3, 4, 5, 6, 7, 8, 9]),
        new Uint8Array([10])
    ])

    const actualHex = bytesToHex(actualBytes)
    expect(actualHex).toBe(expectedHex)

    const decodedMessages = decodeBatchMessage(actualBytes)
    expect(decodedMessages).toEqual([
        new Uint8Array([1, 2, 3, 4, 5, 6, 7, 8, 9]),
        new Uint8Array([10])
    ])
})

test('Decode specific batch message with expected length and single message', () => {
    const hexString = "0000000100000089000000005479cf16a8d46875c9a891cc23524490ed2be9036c6169ff927f1c9fddac6ce0c20000019222d2dad300000000000039790000000063902cef56c3d09d9b637f931f1978a158084924906d9a39c928de8d81942807000000040000000000000000000000640000000000000064000000000000006400000000000000640000000000000064";
    const bytes = Uint8Array.from(Buffer.from(hexString, 'hex'));

    const decodedMessages = decodeBatchMessage(bytes);
    expect(decodedMessages).toHaveLength(0x00000001);
    expect(decodedMessages?.[0]?.length).toBe(0x00000089);
});
