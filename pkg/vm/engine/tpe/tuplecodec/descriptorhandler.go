// Copyright 2021 Matrix Origin
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

package tuplecodec

import (
	"encoding/json"
	"errors"
	"github.com/matrixorigin/matrixone/pkg/vm/engine/tpe/descriptor"
	"github.com/matrixorigin/matrixone/pkg/vm/engine/tpe/orderedcodec"
	"math"
)

var (
	errorTypeInValueNotEqualToTypeInAttribute = errors.New("the type in the value is not equal to the value in the attribute")
	errorDoNotFindTheDesc = errors.New("do not find the descriptor")
	errorDoNotFindTheValue = errors.New("do not find the value")
	errorDecodeDescriptorFailed = errors.New("decode the descriptor failed")
	errorDescriptorSavedIsNotTheWanted = errors.New("the descriptor saved is not the wanted one")
)
/*
Internal descriptor table for schema management.
Attribute  PrimaryKey
---------------------
parentID        Y
ID              Y
Name            N
DescriptorBytes N
 */

var _ descriptor.DescriptorHandler = &DescriptorHandlerImpl{}

type DescriptorHandlerImpl struct {
	codecHandler *TupleCodecHandler
	kvHandler    KVHandler
	serializer ValueSerializer
	kvLimit uint64
}

func NewDescriptorHandlerImpl(codec*TupleCodecHandler,
		kv KVHandler,
		vs ValueSerializer,
		limit uint64) *DescriptorHandlerImpl {
	return &DescriptorHandlerImpl{
		codecHandler: codec,
		kvHandler: kv,
		serializer: vs,
		kvLimit: limit,
	}
}

//encodeFieldsIntoValue the value(parentID,ID,Name,Bytes)
func (dhi *DescriptorHandlerImpl) encodeFieldsIntoValue(parentID uint64,
		ID uint64,name string,descBytes []byte) (TupleValue,error) {
	//serialize the value(parentID,ID,Name,Bytes)
	var fields []interface{}
	fields = append(fields,parentID)
	fields = append(fields,ID)
	fields = append(fields,name)
	fields = append(fields,descBytes)

	out := TupleValue{}
	for i := 0; i < len(fields); i++ {
		serialized, _, err := dhi.serializer.SerializeValue(out,fields[i])
		if err != nil {
			return nil, err
		}
		out = serialized
	}
	return out,nil
}

//encodeRelationDescIntoValue the relationDesc into the value(parentID,ID,Name,Bytes)
func (dhi *DescriptorHandlerImpl) encodeRelationDescIntoValue(parentID uint64,
		desc *descriptor.RelationDesc) (TupleValue,error) {
	//marshal desc
	descBytes,err := MarshalRelationDesc(desc)
	if err != nil {
		return nil, err
	}

	return dhi.encodeFieldsIntoValue(parentID, uint64(desc.ID),desc.Name,descBytes)
}

//encodeDatabaseDescIntoValue the databaseDesc into the value(parentID,ID,Name,Bytes)
func (dhi *DescriptorHandlerImpl) encodeDatabaseDescIntoValue(parentID uint64,
		desc *descriptor.DatabaseDesc) (TupleValue,error) {
	//marshal desc
	descBytes,err := MarshalDatabaseDesc(desc)
	if err != nil {
		return nil, err
	}

	return dhi.encodeFieldsIntoValue(parentID, uint64(desc.ID),desc.Name,descBytes)
}

// decodeValue decodes the data from (parentID,ID,Name,Bytes)
func (dhi *DescriptorHandlerImpl) decodeValue(data []byte) ([]*orderedcodec.DecodedItem,error) {
	attrCnt := len(internalDescriptorTableDesc.Attributes)
	dis := make([]*orderedcodec.DecodedItem,0,attrCnt)
	for j := 0; j < attrCnt; j++ {
		rest, di, err := dhi.serializer.DeserializeValue(data)
		if err != nil {
			return nil, err
		}
		dis = append(dis,di)
		data = rest
	}
	return dis,nil
}

// MakePrefixWithParentID makes the prefix(tenantID,dbID,tableID,indexID,parentID)
func (dhi *DescriptorHandlerImpl) MakePrefixWithOneExtraID(dbID uint64, tableID uint64, indexID uint64, extraID uint64) ([]byte, *orderedcodec.EncodedItem) {
	tke := dhi.codecHandler.GetEncoder()

	//make prefix
	var prefix TupleKey
	// tenantID,dbID,tableID,indexID
	prefix,_ = tke.EncodeIndexPrefix(prefix,
		dbID,
		tableID,
		indexID)

	// append parentID
	prefix,_ = tke.oe.EncodeUint64(prefix, extraID)
	return prefix,nil
}

// makePrefixWithParentIDAndTableID makes the prefix(tenantID,dbID,tableID,indexID,parentID,tableID)
func (dhi *DescriptorHandlerImpl) makePrefixWithParentIDAndTableID(dbID uint64,
		tableID uint64, indexID uint64, parentID uint64,ID uint64)(TupleKey, *orderedcodec.EncodedItem){
	prefix,_ := dhi.MakePrefixWithOneExtraID(dbID,tableID,indexID,parentID)

	tke := dhi.codecHandler.GetEncoder()

	// append tableID
	prefix,_ = tke.oe.EncodeUint64(prefix,ID)
	return prefix,nil
}

// getValueByName gets the value for the key by the name
func (dhi *DescriptorHandlerImpl) GetValuesWithPrefix(parentID uint64, callbackCtx interface{}, callback func(callbackCtx interface{}, dis []*orderedcodec.DecodedItem) ([]byte, error)) ([]byte, error) {
	/*
		1,make prefix (tenantID,dbID,tableID,indexID,parentID)
		2,get keys with the prefix
		3,decode the ID and the Name and find the desired name
	*/

	//make prefix
	var prefix TupleKey
	// append parentID
	prefix,_ = dhi.MakePrefixWithOneExtraID(InternalDatabaseID,
		InternalDescriptorTableID,
		uint64(PrimaryIndexID),
		parentID)

	//get keys with the prefix
	prefixLen := len(prefix)
	for {
		keys, values, err := dhi.kvHandler.GetWithPrefix(prefix,prefixLen,dhi.kvLimit)
		if err != nil {
			return nil, err
		}

		if len(keys) == 0 {
			break
		}

		for i := 0; i < len(keys); i++ {
			//decode the name which is in the value
			data := values[i]
			//decode the data into (parentID,ID,Name,Bytes)
			dis,err := dhi.decodeValue(data)
			if err != nil {
				return nil, err
			}

			//exec callback
			bytesInValues,err := callback(callbackCtx,dis)
			if err != nil {
				return nil, err
			}

			if bytesInValues != nil {
				return bytesInValues, nil
			}
		}

		//get the next prefix
		prefix = SuccessorOfKey(keys[len(keys) - 1])
	}

	return nil, errorDoNotFindTheDesc
}

//callbackForGetTableDescByName extracts the tabledesc by name
func (dhi *DescriptorHandlerImpl) callbackForGetTableDescByName(callbackCtx interface{},dis []*orderedcodec.DecodedItem)([]byte,error) {
	//get the name and the desc
	nameAttr := internalDescriptorTableDesc.Attributes[InternalDescriptorTableID_name_ID]
	descAttr := internalDescriptorTableDesc.Attributes[InternalDescriptorTableID_desc_ID]
	nameDI := dis[InternalDescriptorTableID_name_ID]
	descDI := dis[InternalDescriptorTableID_desc_ID]
	if !(nameDI.IsValueType(nameAttr.Ttype) ||
		descDI.IsValueType(descAttr.Ttype)) {
		return nil,errorTypeInValueNotEqualToTypeInAttribute
	}

	if nameInValue,ok := nameDI.Value.(string); ok {
		name := callbackCtx.(string)
		//check the name
		if name == nameInValue {//get it
			//deserialize the desc
			if bytesInValue,ok2 := descDI.Value.([]byte); ok2 {
				return bytesInValue,nil
			}
		}
	}
	return nil, nil
}

// getValueByName gets the value for the key by the name
func (dhi *DescriptorHandlerImpl) getValueByName(parentID uint64, name string) ([]byte,error) {
	return dhi.GetValuesWithPrefix(parentID, name, dhi.callbackForGetTableDescByName)
}

func (dhi *DescriptorHandlerImpl) LoadRelationDescByName(parentID uint64, name string) (*descriptor.RelationDesc, error) {
	/*
	1,make prefix (tenantID,dbID,tableID,indexID,parentID)
	2,get keys with the prefix
	3,decode the ID and the Name and find the desired name
	 */

	bytesInValue,err := dhi.getValueByName(parentID,name)
	if err != nil {
		return nil, err
	}
	desc, err := UnmarshalRelationDesc(bytesInValue)
	if err != nil {
		return nil, err
	}
	return desc,nil
}

func (dhi *DescriptorHandlerImpl) LoadRelationDescByID(parentID uint64, tableID uint64) (*descriptor.RelationDesc, error) {
	/*
		1,make prefix (tenantID,dbID,tableID,indexID,parentID,tableID)
		2,get keys with the key
		3,decode the ID and the Name and find the desired name
	*/
	key,_ := dhi.makePrefixWithParentIDAndTableID(InternalDatabaseID,
		InternalDescriptorTableID,
		uint64(PrimaryIndexID),
		parentID,
		tableID)

	value, err := dhi.kvHandler.Get(key)
	if err != nil {
		return nil, err
	}

	dis,err := dhi.decodeValue(value)
	if err != nil {
		return nil, err
	}
	descAttr := internalDescriptorTableDesc.Attributes[InternalDescriptorTableID_desc_ID]
	descDI := dis[InternalDescriptorTableID_desc_ID]
	if !descDI.IsValueType(descAttr.Ttype) {
		return nil,errorTypeInValueNotEqualToTypeInAttribute
	}

	//deserialize the desc
	if bytesInValue,ok := descDI.Value.([]byte); ok {
		tableDesc,err := UnmarshalRelationDesc(bytesInValue)
		if err != nil {
			return nil, err
		}
		return tableDesc,nil
	}
	return nil,errorDoNotFindTheDesc
}

func (dhi *DescriptorHandlerImpl) StoreRelationDescByName(parentID uint64, name string, tableDesc *descriptor.RelationDesc) error {
	desc, err := dhi.LoadRelationDescByName(parentID,name)
	if err != nil {
		//if err is not found
		//save the descriptor
		if err == errorDoNotFindTheDesc {
			err := dhi.StoreRelationDescByID(parentID, uint64(tableDesc.ID),tableDesc)
			if err != nil {
				return err
			}
			return nil
		}
		return err
	}
	if desc.Name != name ||
			desc.ID != tableDesc.ID ||
			desc.Name != tableDesc.Name {
		return errorDescriptorSavedIsNotTheWanted
	}
	//get the id, update it and save the descriptor
	err = dhi.StoreRelationDescByID(parentID, uint64(tableDesc.ID),tableDesc)
	if err != nil {
		return err
	}
	return nil
}

//StoreRelationDescByID saves the descriptor.
//It will overwrite the exists one.
func (dhi *DescriptorHandlerImpl) StoreRelationDescByID(parentID uint64, tableID uint64, tableDesc *descriptor.RelationDesc) error {
	/*
		1,make prefix (tenantID,dbID,tableID,indexID,parentID,tableID)
		2,serialize the value with new descriptor
		3,save the key with value
	*/
	key,_ := dhi.makePrefixWithParentIDAndTableID(InternalDatabaseID,
		InternalDescriptorTableID,
		uint64(PrimaryIndexID),
		parentID,
		tableID)

	//serialize the value with new descriptor
	//the value (parentID,ID,name,desc)
	value,err := dhi.encodeRelationDescIntoValue(parentID,tableDesc)
	if err != nil {
		return err
	}

	err = dhi.kvHandler.Set(key, value)
	if err != nil {
		return err
	}
	return nil
}

func (dhi *DescriptorHandlerImpl) DeleteRelationDescByID(parentID uint64, tableID uint64) error {
	/*
		1,make prefix (tenantID,dbID,tableID,indexID,parentID,tableID)
		2,serialize the value with new descriptor
		3,save the key with value
	*/
	key,_ := dhi.makePrefixWithParentIDAndTableID(InternalDatabaseID,
		InternalDescriptorTableID,
		uint64(PrimaryIndexID),
		parentID,
		tableID)

	err := dhi.kvHandler.Delete(key)
	if err != nil {
		return err
	}
	return nil
}

func (dhi *DescriptorHandlerImpl) LoadDatabaseDescByName(name string) (*descriptor.DatabaseDesc, error) {
	/*
		1,make prefix (tenantID,dbID,tableID,indexID,parentID(-1))
		2,get keys with the prefix
		3,decode the ID and the Name and find the desired name
	*/

	bytesInValue,err := dhi.getValueByName(math.MaxUint64,name)
	if err != nil {
		return nil, err
	}
	desc, err := UnmarshalDatabaseDesc(bytesInValue)
	if err != nil {
		return nil, err
	}
	return desc,nil
}

func (dhi *DescriptorHandlerImpl) LoadDatabaseDescByID(dbID uint64) (*descriptor.DatabaseDesc, error) {
	/*
		1,make prefix (tenantID,dbID,tableID,indexID,parentID,tableID)
		2,get keys with the key
		3,decode the ID and the Name and find the desired name
	*/
	key,_ := dhi.makePrefixWithParentIDAndTableID(InternalDatabaseID,
		InternalDescriptorTableID,
		uint64(PrimaryIndexID),
		math.MaxUint64,
		dbID)

	value, err := dhi.kvHandler.Get(key)
	if err != nil {
		return nil, err
	}

	dis,err := dhi.decodeValue(value)
	if err != nil {
		return nil, err
	}
	descAttr := internalDescriptorTableDesc.Attributes[InternalDescriptorTableID_desc_ID]
	descDI := dis[InternalDescriptorTableID_desc_ID]
	if !descDI.IsValueType(descAttr.Ttype) {
		return nil,errorTypeInValueNotEqualToTypeInAttribute
	}

	//deserialize the desc
	if bytesInValue,ok := descDI.Value.([]byte); ok {
		dbDesc,err := UnmarshalDatabaseDesc(bytesInValue)
		if err != nil {
			return nil, err
		}
		return dbDesc,nil
	}
	return nil,errorDoNotFindTheDesc
}

func (dhi *DescriptorHandlerImpl) StoreDatabaseDescByName(name string, dbDesc *descriptor.DatabaseDesc) error {
	desc, err := dhi.LoadRelationDescByName(math.MaxUint64,name)
	if err != nil {
		//if err is not found
		//save the descriptor
		if err == errorDoNotFindTheDesc {
			err := dhi.StoreDatabaseDescByID(uint64(dbDesc.ID),dbDesc)
			if err != nil {
				return err
			}
			return nil
		}
		return err
	}
	if desc.Name != name ||
		desc.ID != dbDesc.ID ||
		desc.Name != dbDesc.Name {
		return errorDescriptorSavedIsNotTheWanted
	}
	//get the id, update it and save the descriptor
	err = dhi.StoreDatabaseDescByID(uint64(dbDesc.ID),dbDesc)
	if err != nil {
		return err
	}
	return nil
}

//StoreDatabaseDescByID saves the descriptor.
//It will overwrite the exists one.
func (dhi *DescriptorHandlerImpl) StoreDatabaseDescByID(dbID uint64, dbDesc *descriptor.DatabaseDesc) error {
	/*
		1,make prefix (tenantID,dbID,tableID,indexID,parentID,tableID)
		2,serialize the value with new descriptor
		3,save the key with value
	*/
	key,_ := dhi.makePrefixWithParentIDAndTableID(InternalDatabaseID,
		InternalDescriptorTableID,
		uint64(PrimaryIndexID),
		math.MaxUint64,
		dbID)

	//serialize the value with new descriptor
	//the value (parentID,ID,name,desc)
	value,err := dhi.encodeDatabaseDescIntoValue(math.MaxUint64,dbDesc)
	if err != nil {
		return err
	}

	err = dhi.kvHandler.Set(key, value)
	if err != nil {
		return err
	}
	return nil
}

func (dhi *DescriptorHandlerImpl) DeleteDatabaseDescByID(dbID uint64) error {
	/*
		1,make prefix (tenantID,dbID,tableID,indexID,parentID,tableID)
		2,serialize the value with new descriptor
		3,save the key with value
	*/
	key,_ := dhi.makePrefixWithParentIDAndTableID(InternalDatabaseID,
		InternalDescriptorTableID,
		uint64(PrimaryIndexID),
		math.MaxUint64,
		dbID)

	err := dhi.kvHandler.Delete(key)
	if err != nil {
		return err
	}
	return nil
}

func (dhi *DescriptorHandlerImpl) encodeAsyncgcValue(epoch uint64, dbID uint64, tableID uint64, desc *descriptor.RelationDesc) (TupleValue,error) {
	//marshal desc
	descBytes,err := MarshalRelationDesc(desc)
	if err != nil {
		return nil, err
	}

	//serialize the value(epoch,dbid,tableid,Bytes)
	var fields []interface{}
	fields = append(fields,epoch)
	fields = append(fields,dbID)
	fields = append(fields,tableID)
	fields = append(fields,descBytes)

	out := TupleValue{}
	for i := 0; i < len(fields); i++ {
		serialized, _, err := dhi.serializer.SerializeValue(out,fields[i])
		if err != nil {
			return nil, err
		}
		out = serialized
	}
	return out,nil
}

func (dhi *DescriptorHandlerImpl) StoreRelationDescIntoAsyncGC(epoch uint64, dbID uint64, desc *descriptor.RelationDesc) error {
	//save thing into the internal async gc (epoch(pk),dbid,tableid,desc)
	//prefix(tenantID,dbID,tableID,indexID,epoch)
	var key TupleKey
	key,_ = dhi.MakePrefixWithOneExtraID(InternalDatabaseID,
		InternalAsyncGCTableID,
		uint64(PrimaryIndexID),
		epoch)

	value, err := dhi.encodeAsyncgcValue(epoch,dbID, uint64(desc.ID),desc)
	if err != nil {
		return err
	}

	err = dhi.kvHandler.Set(key,value)
	if err != nil {
		return err
	}

	return nil
}

func (dhi *DescriptorHandlerImpl) ListRelationDescFromAsyncGC(epoch uint64) ([]*descriptor.RelationDesc, error) {
	//TODO:
	return nil, nil
}


//MarshalRelationDesc encods the relationDesc into the bytes
func MarshalRelationDesc(desc *descriptor.RelationDesc) ([]byte,error) {
	marshal, err := json.Marshal(*desc)
	if err != nil {
		return nil,err
	}
	return marshal,nil
}

//MarshalDatabaseDesc encods the relationDesc into the bytes
func MarshalDatabaseDesc(desc *descriptor.DatabaseDesc) ([]byte,error) {
	marshal, err := json.Marshal(*desc)
	if err != nil {
		return nil,err
	}
	return marshal,nil
}

// UnmarshalRelationDesc decodes the bytes into the relationDesc
func UnmarshalRelationDesc(data []byte) (*descriptor.RelationDesc,error) {
	tableDesc := &descriptor.RelationDesc{}
	err := json.Unmarshal(data, tableDesc)
	if err != nil {
		return nil, errorDecodeDescriptorFailed
	}
	return tableDesc,nil
}

// UnmarshalDatabaseDesc decodes the bytes into the databaseDesc
func UnmarshalDatabaseDesc(data []byte) (*descriptor.DatabaseDesc,error) {
	dbDesc := &descriptor.DatabaseDesc{}
	err := json.Unmarshal(data, dbDesc)
	if err != nil {
		return nil, errorDecodeDescriptorFailed
	}
	return dbDesc,nil
}