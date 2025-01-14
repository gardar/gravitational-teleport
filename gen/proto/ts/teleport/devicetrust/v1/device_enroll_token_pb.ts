/* eslint-disable */
// @generated by protobuf-ts 2.9.3 with parameter long_type_number,eslint_disable,add_pb_suffix,client_grpc1,server_grpc1,ts_nocheck
// @generated from protobuf file "teleport/devicetrust/v1/device_enroll_token.proto" (package "teleport.devicetrust.v1", syntax proto3)
// tslint:disable
// @ts-nocheck
//
// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
import type { BinaryWriteOptions } from "@protobuf-ts/runtime";
import type { IBinaryWriter } from "@protobuf-ts/runtime";
import { WireType } from "@protobuf-ts/runtime";
import type { BinaryReadOptions } from "@protobuf-ts/runtime";
import type { IBinaryReader } from "@protobuf-ts/runtime";
import { UnknownFieldHandler } from "@protobuf-ts/runtime";
import type { PartialMessage } from "@protobuf-ts/runtime";
import { reflectionMergePartial } from "@protobuf-ts/runtime";
import { MessageType } from "@protobuf-ts/runtime";
import { Timestamp } from "../../../google/protobuf/timestamp_pb";
/**
 * DeviceEnrollToken encapsulates the permission, granted by a device
 * administrator to an user, to enroll a device.
 * An enrolled device allows its user to perform device-aware actions.
 *
 * @generated from protobuf message teleport.devicetrust.v1.DeviceEnrollToken
 */
export interface DeviceEnrollToken {
    /**
     * Opaque enrollement token required by the EnrollDevice RPC.
     *
     * @generated from protobuf field: string token = 1;
     */
    token: string;
    /**
     * Expiration time for the token.
     *
     * @generated from protobuf field: google.protobuf.Timestamp expire_time = 2;
     */
    expireTime?: Timestamp;
}
// @generated message type with reflection information, may provide speed optimized methods
class DeviceEnrollToken$Type extends MessageType<DeviceEnrollToken> {
    constructor() {
        super("teleport.devicetrust.v1.DeviceEnrollToken", [
            { no: 1, name: "token", kind: "scalar", T: 9 /*ScalarType.STRING*/ },
            { no: 2, name: "expire_time", kind: "message", T: () => Timestamp }
        ]);
    }
    create(value?: PartialMessage<DeviceEnrollToken>): DeviceEnrollToken {
        const message = globalThis.Object.create((this.messagePrototype!));
        message.token = "";
        if (value !== undefined)
            reflectionMergePartial<DeviceEnrollToken>(this, message, value);
        return message;
    }
    internalBinaryRead(reader: IBinaryReader, length: number, options: BinaryReadOptions, target?: DeviceEnrollToken): DeviceEnrollToken {
        let message = target ?? this.create(), end = reader.pos + length;
        while (reader.pos < end) {
            let [fieldNo, wireType] = reader.tag();
            switch (fieldNo) {
                case /* string token */ 1:
                    message.token = reader.string();
                    break;
                case /* google.protobuf.Timestamp expire_time */ 2:
                    message.expireTime = Timestamp.internalBinaryRead(reader, reader.uint32(), options, message.expireTime);
                    break;
                default:
                    let u = options.readUnknownField;
                    if (u === "throw")
                        throw new globalThis.Error(`Unknown field ${fieldNo} (wire type ${wireType}) for ${this.typeName}`);
                    let d = reader.skip(wireType);
                    if (u !== false)
                        (u === true ? UnknownFieldHandler.onRead : u)(this.typeName, message, fieldNo, wireType, d);
            }
        }
        return message;
    }
    internalBinaryWrite(message: DeviceEnrollToken, writer: IBinaryWriter, options: BinaryWriteOptions): IBinaryWriter {
        /* string token = 1; */
        if (message.token !== "")
            writer.tag(1, WireType.LengthDelimited).string(message.token);
        /* google.protobuf.Timestamp expire_time = 2; */
        if (message.expireTime)
            Timestamp.internalBinaryWrite(message.expireTime, writer.tag(2, WireType.LengthDelimited).fork(), options).join();
        let u = options.writeUnknownFields;
        if (u !== false)
            (u == true ? UnknownFieldHandler.onWrite : u)(this.typeName, message, writer);
        return writer;
    }
}
/**
 * @generated MessageType for protobuf message teleport.devicetrust.v1.DeviceEnrollToken
 */
export const DeviceEnrollToken = new DeviceEnrollToken$Type();
