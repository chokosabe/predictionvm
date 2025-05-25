import { sha256 } from '@noble/hashes/sha256';
import { parse } from 'lossless-json'
import { base64 } from '@scure/base';
import ABIsABI from './testdata/abi.abi.json'
import { bytesToHex, hexToBytes } from '@noble/hashes/utils';
import { ActionData, TransactionPayload } from './types';

export const ED25519_AUTH_ID = 0x00

export type VMABI = {
    actions: TypedStructABI[]
    outputs: TypedStructABI[]
    types: TypeABI[]
}

type TypedStructABI = {
    id: number
    name: string
}

type TypeABI = {
    name: string,
    fields: ABIField[]
}

type ABIField = {
    name: string,
    type: string
}

export class Marshaler {
    constructor(private abi: VMABI) {
        if (!Array.isArray(this.abi?.actions) || !Array.isArray(this.abi?.outputs)) {
            throw new Error('Invalid ABI')
        }
    }

    public getHash(): Uint8Array {
        const abiAbiMarshaler = new Marshaler(ABIsABI)
        const abiBytes = abiAbiMarshaler.encode("ABI", JSON.stringify(this.abi))
        return sha256(abiBytes)
    }

    public encode(typeName: string, dataJSON: string): Uint8Array {
        const data = parse(dataJSON) as Record<string, unknown>;
        return this.encodeField(typeName, data);
    }

    public encodeTyped(typeName: string, dataJSON: string): Uint8Array {
        const data = parse(dataJSON) as Record<string, unknown>
        const typeABI = this.abi.types.find(type => type.name === typeName)

        if (!typeABI) {
            throw new Error(`Type ${typeName} not found in ABI`)
        }

        // Check for extra fields
        const extraFields = Object.keys(data).filter(key => !typeABI.fields.some(field => field.name === key))
        if (extraFields.length > 0) {
            throw new Error(`Extra fields found in data: ${extraFields.join(', ')}`)
        }

        const typeId = [...this.abi.actions, ...this.abi.outputs].find(typ => typ.name === typeName)?.id

        if (typeId === undefined) {
            throw new Error(`Type ID not found for ${typeName}`)
        }

        const encodedData = this.encodeField(typeName, data)
        return new Uint8Array([typeId, ...encodedData])
    }

    public parseTyped(binary: Uint8Array, typeCategory: 'action' | 'output'): [Record<string, unknown>, number] {
        if (binary.length === 0) {
            throw new Error('Empty binary data')
        }

        const typeId = binary[0]
        const data = binary.slice(1)

        const typeList = typeCategory === 'action' ? this.abi.actions : this.abi.outputs
        const foundType = typeList.find(typ => typ.id === typeId)

        if (!foundType) {
            console.log(typeList)
            throw new Error(`No ${typeCategory} found for id ${typeId}`)
        }

        return this.parse(foundType.name, data) as [Record<string, unknown>, number]
    }

    public parse(outputType: string, actionResultBinary: Uint8Array): [Record<string, unknown>, number] {
        // Handle primitive types
        if (isPrimitiveType(outputType)) {
            return this.decodeField(outputType, actionResultBinary);
        }

        // Handle struct types
        let structABI = this.abi.types.find((type) => type.name === outputType);
        if (!structABI) {
            throw new Error(`No struct ABI found for type ${outputType}`);
        }

        let result: Record<string, unknown> = {};
        let offset = 0;

        for (const field of structABI.fields) {
            const fieldType = field.type;

            // Decode field based on type
            const [decodedValue, bytesConsumed] = this.decodeField(fieldType, actionResultBinary.subarray(offset));
            result[field.name] = decodedValue;
            offset += bytesConsumed;
        }

        return [result, offset];
    }

    public decodeField<T>(type: string, binaryData: Uint8Array): [T, number] {
        // Decodes field and returns value and the number of bytes consumed.
        switch (type) {
            case "bool":
                return decodeBool(binaryData) as [T, number]
            case "[]uint8":
                return decodeBytes(binaryData) as [T, number]
            case "uint8":
            case "uint16":
            case "uint32":
            case "uint64":
            case "uint256":
                return [decodeNumber(type, binaryData) as T, getByteSize(type)];
            case "string":
                return decodeString(binaryData) as [T, number]
            case "Address":
                return decodeAddress(binaryData) as [T, number]
            case "int8":
            case "int16":
            case "int32":
            case "int64":
                return [decodeNumber(type, binaryData) as T, getByteSize(type)];
            default:
                // Handle arrays and structs
                if (type.startsWith('[]')) {
                    return this.decodeSlice(type.slice(2), binaryData) as [T, number]
                } else if (type.startsWith('[')) {
                    //parse [length]type
                    const match = type.match(/^\[(\d+)\](.+)$/);
                    if (match && match[1] && match[2]) {
                        const length = parseInt(match[1], 10);
                        const elementType = match[2];
                        return this.decodeArray(elementType, binaryData, length) as [T, number]
                    } else {
                        throw new Error(`Unsupported type: ${type}`);
                    }
                } else {
                    // Struct type
                    const [decodedStruct, _] = this.parse(type, binaryData);
                    const bytesConsumed = this.getStructByteSize(type, binaryData);
                    return [decodedStruct as T, bytesConsumed];
                }
        }
    }

    private decodeSlice(type: string, binaryData: Uint8Array): [unknown[], number] {
        const length = decodeNumber("uint32", binaryData) as number;
        const [resultArray, offset] = this.decodeArray(type, binaryData.subarray(4), length);
        return [resultArray, offset + 4]
    }

    private decodeArray(type: string, binaryData: Uint8Array, length: number): [unknown[], number] {
        let offset = 0;
        let resultArray = [];
        for (let i = 0; i < length; i++) {
            const [decodedValue, bytesConsumed] = this.decodeField(type, binaryData.subarray(offset));
            resultArray.push(decodedValue);
            offset += bytesConsumed;
        }
        return [resultArray, offset];
    }

    private getStructByteSize(type: string, binaryData: Uint8Array): number {
        const structABI = this.abi.types.find((t) => t.name === type);
        if (!structABI) {
            throw new Error(`No struct ABI found for type ${type}`);
        }

        let totalSize = 0;
        for (const field of structABI.fields) {
            const [_, bytesConsumed] = this.decodeField(field.type, binaryData.subarray(totalSize));
            totalSize += bytesConsumed;
        }
        return totalSize;
    }

    public encodeTransaction(tx: TransactionPayload): Uint8Array {
        if (tx.base.timestamp.slice(-3) !== "000") {
            tx.base.timestamp = String(Math.floor(parseInt(tx.base.timestamp) / 1000) * 1000)
        }

        const timestampBytes = encodeNumber("uint64", tx.base.timestamp);
        const chainIdBytes = encodeNumber("uint256", tx.base.chainId);
        const maxFeeBytes = encodeNumber("uint64", tx.base.maxFee);
        const actionsCountBytes = encodeNumber("uint8", tx.actions.length);

        let actionsBytes = new Uint8Array();
        for (const action of tx.actions) {
            const actionTypeIdBytes = encodeNumber("uint8", this.getActionTypeId(action.actionName));
            const actionDataBytes = this.encodeField(action.actionName, action.data);
            actionsBytes = new Uint8Array([...actionsBytes, ...actionTypeIdBytes, ...actionDataBytes]);
        }

        // const abiHashBytes = this.getHash()

        return new Uint8Array([
            // ...abiHashBytes //TODO: add abi hash to the end of the signable body of transaction
            ...timestampBytes,
            ...chainIdBytes,
            ...maxFeeBytes,
            ...actionsCountBytes,
            ...actionsBytes,
        ]);
    }

    public decodeTransaction(tx: Uint8Array): [TransactionPayload, number] {
        let offset = 0
        let timestamp: bigint
        let bytesConsumed: number

        [timestamp, bytesConsumed] = this.decodeField<bigint>("uint64", tx.slice(offset))
        offset += bytesConsumed

        let chainIdBase64: string
        [chainIdBase64, bytesConsumed] = this.decodeField<string>("[32]uint8", tx.slice(offset))
        offset += bytesConsumed

        let maxFee: bigint
        [maxFee, bytesConsumed] = this.decodeField<bigint>("uint64", tx.slice(offset))
        offset += bytesConsumed

        let actionsCount: number
        [actionsCount, bytesConsumed] = this.decodeField<number>("uint8", tx.slice(offset))
        offset += bytesConsumed

        let actions: ActionData[] = []
        for (let i = 0; i < actionsCount; i++) {
            const [action, bytesConsumed] = this.parseTyped(tx.slice(offset), "action")
            actions.push(action as ActionData)
            offset += bytesConsumed
        }

        return [{
            base: {
                timestamp: timestamp.toString(),
                chainId: chainIdBase64,//FIXME: might be a mistake here
                maxFee: maxFee.toString(),
            },
            actions
        }, offset]
    }

    public getActionTypeId(actionName: string): number {
        const actionABI = this.abi.actions.find(action => action.name === actionName)
        if (!actionABI) throw new Error(`No action ABI found: ${actionName}`)
        return actionABI.id
    }

    private encodeField(type: string, value: unknown, parentActionName?: string): Uint8Array {
        if (type === 'Address' && typeof value === 'string') {
            return encodeAddress(value)
        }

        if ((type === '[]uint8') && typeof value === 'string') {
            const byteArray = Array.from(atob(value), char => char.charCodeAt(0)) as number[]
            return new Uint8Array([...encodeNumber("uint32", byteArray.length), ...byteArray])
        }

        if (type.startsWith('[]')) {
            return this.encodeSlice(type.slice(2), value as unknown[]);
        } else if (type.startsWith('[')) {
            //parse [length]type
            const match = type.match(/^\[(\d+)\](.+)$/);
            if (match && match[1] && match[2]) {
                const length = parseInt(match[1], 10);
                const elementType = match[2];
                return this.encodeArray(elementType, value as unknown[], length);
            } else {
                throw new Error(`Unsupported type: ${type}`);
            }
        }

        switch (type) {
            case "bool":
                return encodeBool(value as boolean)
            case "uint8":
            case "uint16":
            case "uint32":
            case "uint64":
            case "int8":
            case "int16":
            case "int32":
            case "int64":
                return encodeNumber(type, value as number | string)
            case "string":
                return encodeString(value as string)
            default:
                {
                    let structABI: TypeABI | null = null
                    for (const typ of this.abi.types) {
                        if (typ.name === type) {
                            structABI = typ
                            break
                        }
                    }
                    if (!structABI) throw new Error(`No struct ${type} found in action ${type} ABI`)

                    const dataRecord = value as Record<string, unknown>;
                    let resultingBinary = new Uint8Array()
                    for (const field of structABI.fields) {
                        const fieldBinary = this.encodeField(field.type, dataRecord[field.name], type);
                        resultingBinary = new Uint8Array([...resultingBinary, ...fieldBinary])
                    }
                    return resultingBinary
                }
        }
    }

    private encodeSlice(type: string, value: unknown[]): Uint8Array {
        if (!Array.isArray(value)) {
            throw new Error(`Error in encodeArray: Expected an array for type ${type}, but received ${typeof value} of declared type ${type}`)
        }
        const lengthBytes = encodeNumber("uint32", value.length);
        return new Uint8Array([...lengthBytes, ...this.encodeArray(type, value, value.length)]);
    }

    private encodeArray(type: string, value: unknown[], expectedLength: number): Uint8Array {
        if (value.length !== expectedLength) {
            throw new Error(`Error in encodeArray: Expected an array of length ${expectedLength} for type ${type}, but received an array of length ${value.length}`)
        }
        const encodedItems = value.map(item => this.encodeField(type, item));
        const flattenedItems = encodedItems.reduce((acc, item) => {
            if (item instanceof Uint8Array) {
                return [...acc, ...item];
            } else if (typeof item === 'number') {
                return [...acc, item];
            } else {
                throw new Error(`Unexpected item type in encoded array: ${typeof item}`);
            }
        }, [] as number[]);
        return new Uint8Array(flattenedItems)
    }
}

function isPrimitiveType(type: string): boolean {
    const primitiveTypes = [
        "uint8", "uint16", "uint32", "uint64", "uint256",
        "int8", "int16", "int32", "int64",
        "string", "Address", "[]uint8"
    ];
    return primitiveTypes.includes(type) || type.startsWith('[]');
}

export function decodeNumber(type: string, binaryData: Uint8Array): bigint | number {
    const dataView = new DataView(binaryData.buffer, binaryData.byteOffset, binaryData.byteLength);
    let result: bigint | number;

    switch (type) {
        case "uint8":
            result = dataView.getUint8(0);
            break;
        case "uint16":
            result = dataView.getUint16(0, false);
            break;
        case "uint32":
            result = dataView.getUint32(0, false);
            break;
        case "uint64":
            result = dataView.getBigUint64(0, false);
            break;
        case "int8":
            result = dataView.getInt8(0);
            break;
        case "int16":
            result = dataView.getInt16(0, false);
            break;
        case "int32":
            result = dataView.getInt32(0, false);
            break;
        case "int64":
            result = dataView.getBigInt64(0, false);
            break;
        default:
            throw new Error(`Unsupported number type: ${type}`);
    }

    return result;
}

function getByteSize(type: string): number {
    switch (type) {
        case "uint8": return 1;
        case "uint16": return 2;
        case "uint32": return 4;
        case "uint64": return 8;
        case "uint256": return 32;
        case "int8": return 1;
        case "int16": return 2;
        case "int32": return 4;
        case "int64": return 8;
        default: throw new Error(`Unknown type for byte size: ${type}`);
    }
}

function decodeString(binaryData: Uint8Array): [string, number] {
    const length = decodeNumber("uint16", binaryData) as number;
    const textDecoder = new TextDecoder();
    const stringBytes = binaryData.subarray(2, 2 + length); // Skip the length bytes
    const result: [string, number] = [textDecoder.decode(stringBytes), 2 + length];
    return result
}

export function decodeAddress(binaryData: Uint8Array): [string, number] {
    if (binaryData.length < 33) {
        throw new Error("Decoding address: has to have 33 bytes length")
    }
    const addressBytes = binaryData.subarray(0, 33); // Fixed length for Address (33 bytes)
    const hash = sha256(addressBytes);
    const checksum = hash.slice(-4); // Take last 4 bytes
    return ["0x" + bytesToHex(addressBytes) + bytesToHex(checksum), 33];
}

function decodeBytes(binaryData: Uint8Array): [string, number] {
    const length = decodeNumber("uint32", binaryData) as number;
    const byteArray = binaryData.subarray(4, 4 + length); // Skip the length bytes
    const base64String = base64.encode(byteArray);
    return [base64String, 4 + length];
}

function encodeAddress(value: string): Uint8Array {
    if (!/^0x[0-9a-fA-F]{74}$/.test(value)) {
        throw new Error(`Address must be a 74-character hex string with '0x' prefix: ${value}`);
    }

    // Remove 0x prefix
    const hexWithoutPrefix = value.slice(2);
    const allBytes = hexToBytes(hexWithoutPrefix);

    // Split into address and checksum
    const addressBytes = allBytes.slice(0, 33);
    const providedChecksum = allBytes.slice(33, 37);

    // Calculate expected checksum
    const hash = sha256(addressBytes);
    const expectedChecksum = hash.slice(-4); // Take last 4 bytes

    // Verify checksum
    for (let i = 0; i < 4; i++) {
        if (providedChecksum[i] !== expectedChecksum[i]) {
            throw new Error('Invalid address checksum');
        }
    }

    return addressBytes;
}

export function addressBytesFromPubKey(pubKey: Uint8Array): Uint8Array {
    return new Uint8Array([ED25519_AUTH_ID, ...sha256(pubKey)])
}

export function addressHexFromPubKey(pubKey: Uint8Array): string {
    const addressBytes = addressBytesFromPubKey(pubKey)
    const hash = sha256(addressBytes)
    const checksum = hash.slice(-4) // Take last 4 bytes
    return "0x" + bytesToHex(addressBytes) + bytesToHex(checksum)
}

export function encodeNumber(type: string, value: number | string): Uint8Array {
    let bigValue = BigInt(value)
    let buffer: ArrayBuffer
    let dataView: DataView

    switch (type) {
        case "uint8":
            buffer = new ArrayBuffer(1)
            dataView = new DataView(buffer)
            dataView.setUint8(0, Number(bigValue))
            break
        case "uint16":
            buffer = new ArrayBuffer(2)
            dataView = new DataView(buffer)
            dataView.setUint16(0, Number(bigValue), false)
            break
        case "uint32":
            buffer = new ArrayBuffer(4)
            dataView = new DataView(buffer)
            dataView.setUint32(0, Number(bigValue), false)
            break
        case "uint64":
            buffer = new ArrayBuffer(8)
            dataView = new DataView(buffer)
            dataView.setBigUint64(0, bigValue, false)
            break
        case "uint256":
            buffer = new ArrayBuffer(32)
            dataView = new DataView(buffer)
            for (let i = 0; i < 32; i++) {
                dataView.setUint8(31 - i, Number(bigValue & 255n))
                bigValue >>= 8n
            }
            break
        case "int8":
            buffer = new ArrayBuffer(1)
            dataView = new DataView(buffer)
            dataView.setInt8(0, Number(bigValue))
            break
        case "int16":
            buffer = new ArrayBuffer(2)
            dataView = new DataView(buffer)
            dataView.setInt16(0, Number(bigValue), false)
            break
        case "int32":
            buffer = new ArrayBuffer(4)
            dataView = new DataView(buffer)
            dataView.setInt32(0, Number(bigValue), false)
            break
        case "int64":
            buffer = new ArrayBuffer(8)
            dataView = new DataView(buffer)
            dataView.setBigInt64(0, bigValue, false)
            break
        default:
            throw new Error(`Unsupported number type: ${type}`)
    }

    return new Uint8Array(buffer)
}

function encodeString(value: string): Uint8Array {
    const encoder = new TextEncoder()
    const stringBytes = encoder.encode(value)
    const lengthBytes = encodeNumber("uint16", stringBytes.length)
    return new Uint8Array([...lengthBytes, ...stringBytes])
}

function encodeBool(value: boolean): Uint8Array {
    return new Uint8Array([value ? 1 : 0])
}

function decodeBool(binaryData: Uint8Array): [boolean, number] {
    const val = binaryData[0];
    if (val !== 0 && val !== 1) {
        throw new Error(`Invalid boolean value: ${val}. Expected 0 or 1.`);
    }
    const value = val === 1;
    return [value, 1];
}
