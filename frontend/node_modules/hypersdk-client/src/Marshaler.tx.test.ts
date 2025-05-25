import { bytesToHex } from '@noble/hashes/utils'
import { hexToBytes } from '@noble/curves/abstract/utils'
import { Marshaler, VMABI } from "./Marshaler";
import fs from 'fs'
import { expect, test } from 'vitest';
import { PrivateKeySigner } from './PrivateKeySigner';
import { ed25519 } from '@noble/curves/ed25519';
import { idStringToBigInt } from '.';
import { TransactionPayload } from './types';

test('Empty transaction', () => {
  const chainId = idStringToBigInt("2c7iUW3kCDwRA9ZFd5bjZZc8iDy68uAsFSBahjqSZGttiTDSNH")

  const tx: TransactionPayload = {
    base: {
      timestamp: "1717111222000",
      chainId: String(chainId),
      maxFee: String(10n * (10n ** 9n)),
    },
    actions: [],
  }

  const abiJSON = fs.readFileSync(`./src/testdata/abi.abi.json`, 'utf8')
  const marshaler = new Marshaler(JSON.parse(abiJSON) as VMABI)

  const txDigest = marshaler.encodeTransaction(tx)

  expect(
    bytesToHex(txDigest)
  ).toBe(
    "0000018fcbcdeef0d36e467c73e2840140cc41b3d72f8a5a7446b2399c39b9c74d4cf077d250902400000002540be40000"
  );
})

const minimalABI = {
  "actions": [
    {
      "id": 0,
      "name": "Transfer"
    },
  ],
  "outputs": [],
  "types": [
    {
      "name": "Transfer",
      "fields": [
        {
          "name": "to",
          "type": "Address"
        },
        {
          "name": "value",
          "type": "uint64"
        },
        {
          "name": "memo",
          "type": "[]uint8"
        }
      ]
    },
  ]
}

test('Single action tx sign and marshal', async () => {
  const chainId = idStringToBigInt("2c7iUW3kCDwRA9ZFd5bjZZc8iDy68uAsFSBahjqSZGttiTDSNH");
  const addrString = "0x0102030405060708090a0b0c0d0e0f10111213140000000000000000000000000020db0e6c";

  const marshaler = new Marshaler(minimalABI)

  const actionData = {
    actionName: "Transfer",
    data: {
      to: addrString,
      value: "1000",
      memo: Buffer.from("hi").toString('base64'),
    }
  }

  const tx: TransactionPayload = {
    base: {
      timestamp: "1717111222000",
      chainId: String(chainId),
      maxFee: String(10n * (10n ** 9n)),
    },
    actions: [actionData],
  }

  const digest = marshaler.encodeTransaction(tx)

  const expectedDigest = "0000018fcbcdeef0d36e467c73e2840140cc41b3d72f8a5a7446b2399c39b9c74d4cf077d250902400000002540be40001000102030405060708090a0b0c0d0e0f10111213140000000000000000000000000000000000000003e8000000026869"
  expect(Buffer.from(digest).toString('hex')).toBe(expectedDigest);

  const privateKeyHex = "323b1d8f4eed5f0da9da93071b034f2dce9d2d22692c172f3cb252a64ddfafd01b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7";
  const privateKey = hexToBytes(privateKeyHex).slice(0, 32)

  const pubKey = ed25519.getPublicKey(privateKey)
  expect(bytesToHex(pubKey)).toBe(("1b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7"))

  const privateKeySigner = new PrivateKeySigner(privateKey);

  const signedTxBytes = await privateKeySigner.signTx(tx, minimalABI);

  expect(Buffer.from(signedTxBytes).toString('hex')).toBe(
    expectedDigest +
    "00" +//auth id
    "1b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7" +//pubkey
    "b86baec5fe89f2bb585cb781f694a398107fe760577c750da3e9b381c5f5a3673c4a85c65a0db8d5ed03b4c4fd7f818d99270504e65c0ebf4d73884e0bfce60a"//signature
  );
});
