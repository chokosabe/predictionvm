import { bytesToHex } from '@noble/hashes/utils'
import { Marshaler, VMABI } from "./Marshaler";
import fs from 'fs';
import { expect, test } from 'vitest';
import { isLosslessNumber, parse, stringify } from 'lossless-json';

const testCases: [string, string][] = [
  ["empty", "MockObjectSingleNumber"],
  ["uint16", "MockObjectSingleNumber"],
  ["numbers", "MockObjectAllNumbers"],
  ["arrays", "MockObjectArrays"],
  ["transfer", "MockActionTransfer"],
  ["transferField", "MockActionWithTransfer"],
  ["transfersArray", "MockActionWithTransferArray"],
  ["strBytes", "MockObjectStringAndBytes"],
  ["strByteZero", "MockObjectStringAndBytes"],
  ["strBytesEmpty", "MockObjectStringAndBytes"],
  ["strOnly", "MockObjectStringAndBytes"],
  ["outer", "Outer"],
  ["fixedBytes", "FixedBytes"],
  ["bools", "Bools"],
]

const abiJSON = fs.readFileSync(`./src/testdata/abi.json`, 'utf8')
const marshaler = new Marshaler(JSON.parse(abiJSON) as VMABI)

test('ABI hash', () => {
  const actualHash = marshaler.getHash()
  const actualHex = bytesToHex(actualHash)

  const expectedHex = String(
    fs.readFileSync(`./src/testdata/abi.hash.hex`, 'utf8')
  ).trim()

  expect(actualHex).toBe(expectedHex)
})

for (const [testCase, action] of testCases) {
  test(`${testCase} spec - encode and decode`, () => {
    const expectedHex = String(
      fs.readFileSync(`./src/testdata/${testCase}.hex`, 'utf8')
    ).trim().split("\n").map(line => line.split("//")[0]?.trim()).join("")
    const input = fs.readFileSync(`./src/testdata/${testCase}.json`, 'utf8');

    // Test encoding
    const encodedBinary = marshaler.encode(action, input);
    const actualHex = bytesToHex(encodedBinary);
    expect(actualHex).toEqual(expectedHex);

    // Test decoding
    const [decodedData, _] = marshaler.parse(action, encodedBinary);

    // Compare the decoded data with the original input
    const originalData = parse(input)

    const compareObjects = (obj1: any, obj2: any) => {
      for (const key in obj1) {
        if (typeof obj1[key] === 'bigint' || typeof obj2[key] === 'bigint') {
          expect(String(obj1[key])).toEqual(String(obj2[key]));
        } else {
          const expected = isLosslessNumber(obj2[key]) ? obj2[key].toString() : obj2[key]
          const actual = isLosslessNumber(obj1[key]) ? obj1[key].toString() : obj1[key]
          expect(String(actual)).toEqual(String(expected));
        }
      }
    };

    compareObjects(decodedData, originalData);
    // Use JSON.stringify for string representation comparison
    const stringifiedDecoded = stringify(decodedData);
    const stringifiedOriginal = stringify(originalData);
    expect(stringifiedDecoded).toEqual(stringifiedOriginal);
  });
}
