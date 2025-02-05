// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package mongo

import (
	"errors"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/launchpadcentral/mongo-go-driver/bson"
	"github.com/launchpadcentral/mongo-go-driver/bson/bsoncodec"
	"github.com/launchpadcentral/mongo-go-driver/bson/bsontype"
	"github.com/launchpadcentral/mongo-go-driver/bson/primitive"
	"github.com/launchpadcentral/mongo-go-driver/x/bsonx"
	"github.com/launchpadcentral/mongo-go-driver/x/bsonx/bsoncore"
)

func noerr(t *testing.T, err error) {
	if err != nil {
		t.Helper()
		t.Errorf("Unexpected error: (%T)%v", err, err)
		t.FailNow()
	}
}

func TestTransformDocument(t *testing.T) {
	testCases := []struct {
		name     string
		document interface{}
		want     bsonx.Doc
		err      error
	}{
		{
			"bson.Marshaler",
			bMarsh{bsonx.Doc{{"foo", bsonx.String("bar")}}},
			bsonx.Doc{{"foo", bsonx.String("bar")}},
			nil,
		},
		{
			"reflection",
			reflectStruct{Foo: "bar"},
			bsonx.Doc{{"foo", bsonx.String("bar")}},
			nil,
		},
		{
			"reflection pointer",
			&reflectStruct{Foo: "bar"},
			bsonx.Doc{{"foo", bsonx.String("bar")}},
			nil,
		},
		{
			"unsupported type",
			[]string{"foo", "bar"},
			nil,
			MarshalError{
				Value: []string{"foo", "bar"},
				Err:   errors.New("WriteArray can only write a Array while positioned on a Element or Value but is positioned on a TopLevel")},
		},
		{
			"nil",
			nil,
			nil,
			ErrNilDocument,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := transformDocument(bson.NewRegistryBuilder().Build(), tc.document)
			if !cmp.Equal(err, tc.err, cmp.Comparer(compareErrors)) {
				t.Errorf("Error does not match expected error. got %v; want %v", err, tc.err)
			}

			if diff := cmp.Diff(got, tc.want, cmp.AllowUnexported(bsonx.Elem{}, bsonx.Val{})); diff != "" {
				t.Errorf("Returned documents differ: (-got +want)\n%s", diff)
			}
		})
	}
}

func TestTransformAndEnsureID(t *testing.T) {
	t.Run("newly added _id should be first element", func(t *testing.T) {
		doc := bson.D{{"foo", "bar"}, {"baz", "qux"}, {"hello", "world"}}
		want := bsonx.Doc{
			{"_id", bsonx.Null()}, {"foo", bsonx.String("bar")},
			{"baz", bsonx.String("qux")}, {"hello", bsonx.String("world")},
		}
		got, id, err := transformAndEnsureID(bson.DefaultRegistry, doc)
		noerr(t, err)
		oid, ok := id.(primitive.ObjectID)
		if !ok {
			t.Fatalf("Expected returned id to be a %T, but was %T", primitive.ObjectID{}, id)
		}
		want[0] = bsonx.Elem{"_id", bsonx.ObjectID(oid)}
		if diff := cmp.Diff(got, want, cmp.AllowUnexported(bsonx.Elem{}, bsonx.Val{})); diff != "" {
			t.Errorf("Returned documents differ: (-got +want)\n%s", diff)
		}
	})
	t.Run("existing _id should be first element", func(t *testing.T) {
		doc := bson.D{{"foo", "bar"}, {"baz", "qux"}, {"_id", 3.14159}, {"hello", "world"}}
		want := bsonx.Doc{
			{"_id", bsonx.Double(3.14159)}, {"foo", bsonx.String("bar")},
			{"baz", bsonx.String("qux")}, {"hello", bsonx.String("world")},
		}
		got, id, err := transformAndEnsureID(bson.DefaultRegistry, doc)
		noerr(t, err)
		_, ok := id.(float64)
		if !ok {
			t.Fatalf("Expected returned id to be a %T, but was %T", float64(0), id)
		}
		if diff := cmp.Diff(got, want, cmp.AllowUnexported(bsonx.Elem{}, bsonx.Val{})); diff != "" {
			t.Errorf("Returned documents differ: (-got +want)\n%s", diff)
		}
	})
	t.Run("existing _id as first element should remain first element", func(t *testing.T) {
		doc := bson.D{{"_id", 3.14159}, {"foo", "bar"}, {"baz", "qux"}, {"hello", "world"}}
		want := bsonx.Doc{
			{"_id", bsonx.Double(3.14159)}, {"foo", bsonx.String("bar")},
			{"baz", bsonx.String("qux")}, {"hello", bsonx.String("world")},
		}
		got, id, err := transformAndEnsureID(bson.DefaultRegistry, doc)
		noerr(t, err)
		_, ok := id.(float64)
		if !ok {
			t.Fatalf("Expected returned id to be a %T, but was %T", float64(0), id)
		}
		if diff := cmp.Diff(got, want, cmp.AllowUnexported(bsonx.Elem{}, bsonx.Val{})); diff != "" {
			t.Errorf("Returned documents differ: (-got +want)\n%s", diff)
		}
	})
	t.Run("existing _id should not overwrite a first binary field", func(t *testing.T) {
		doc := bson.D{{"bin", []byte{0, 0, 0}}, {"_id", "LongEnoughIdentifier"}}
		want := bsonx.Doc{
			{"_id", bsonx.String("LongEnoughIdentifier")},
			{"bin", bsonx.Binary(0x00, []byte{0x00, 0x00, 0x00})},
		}
		got, id, err := transformAndEnsureID(bson.DefaultRegistry, doc)
		noerr(t, err)
		_, ok := id.(string)
		if !ok {
			t.Fatalf("Expected returned id to be a %T, but was %T", string(0), id)
		}
		if diff := cmp.Diff(got, want, cmp.AllowUnexported(bsonx.Elem{}, bsonx.Val{})); diff != "" {
			t.Errorf("Returned documents differ: (-got +want)\n%s", diff)
		}
	})
}

func TestTransformAggregatePipeline(t *testing.T) {
	index, arr := bsoncore.AppendArrayStart(nil)
	dindex, arr := bsoncore.AppendDocumentElementStart(arr, "0")
	arr = bsoncore.AppendInt32Element(arr, "$limit", 12345)
	arr, _ = bsoncore.AppendDocumentEnd(arr, dindex)
	arr, _ = bsoncore.AppendArrayEnd(arr, index)

	testCases := []struct {
		name     string
		pipeline interface{}
		arr      bsonx.Arr
		err      error
	}{
		{
			"Pipeline/error",
			Pipeline{{{"hello", func() {}}}}, bsonx.Arr{},
			MarshalError{Value: primitive.D{}, Err: errors.New("no encoder found for func()")},
		},
		{
			"Pipeline/success",
			Pipeline{{{"hello", "world"}}, {{"pi", 3.14159}}},
			bsonx.Arr{
				bsonx.Document(bsonx.Doc{{"hello", bsonx.String("world")}}),
				bsonx.Document(bsonx.Doc{{"pi", bsonx.Double(3.14159)}}),
			},
			nil,
		},
		{
			"bsonx.Arr",
			bsonx.Arr{bsonx.Document(bsonx.Doc{{"$limit", bsonx.Int32(12345)}})},
			bsonx.Arr{bsonx.Document(bsonx.Doc{{"$limit", bsonx.Int32(12345)}})},
			nil,
		},
		{
			"[]bsonx.Doc",
			[]bsonx.Doc{{{"$limit", bsonx.Int32(12345)}}},
			bsonx.Arr{bsonx.Document(bsonx.Doc{{"$limit", bsonx.Int32(12345)}})},
			nil,
		},
		{
			"primitive.A/error",
			primitive.A{"5"},
			bsonx.Arr{},
			MarshalError{Value: string(""), Err: errors.New("WriteString can only write while positioned on a Element or Value but is positioned on a TopLevel")},
		},
		{
			"primitive.A/success",
			primitive.A{bson.D{{"$limit", int32(12345)}}, map[string]interface{}{"$count": "foobar"}},
			bsonx.Arr{
				bsonx.Document(bsonx.Doc{{"$limit", bsonx.Int32(12345)}}),
				bsonx.Document(bsonx.Doc{{"$count", bsonx.String("foobar")}}),
			},
			nil,
		},
		{
			"bson.A/error",
			bson.A{"5"},
			bsonx.Arr{},
			MarshalError{Value: string(""), Err: errors.New("WriteString can only write while positioned on a Element or Value but is positioned on a TopLevel")},
		},
		{
			"bson.A/success",
			bson.A{bson.D{{"$limit", int32(12345)}}, map[string]interface{}{"$count": "foobar"}},
			bsonx.Arr{
				bsonx.Document(bsonx.Doc{{"$limit", bsonx.Int32(12345)}}),
				bsonx.Document(bsonx.Doc{{"$count", bsonx.String("foobar")}}),
			},
			nil,
		},
		{
			"[]interface{}/error",
			[]interface{}{"5"},
			bsonx.Arr{},
			MarshalError{Value: string(""), Err: errors.New("WriteString can only write while positioned on a Element or Value but is positioned on a TopLevel")},
		},
		{
			"[]interface{}/success",
			[]interface{}{bson.D{{"$limit", int32(12345)}}, map[string]interface{}{"$count": "foobar"}},
			bsonx.Arr{
				bsonx.Document(bsonx.Doc{{"$limit", bsonx.Int32(12345)}}),
				bsonx.Document(bsonx.Doc{{"$count", bsonx.String("foobar")}}),
			},
			nil,
		},
		{
			"bsoncodec.ValueMarshaler/MarshalBSONValue error",
			bvMarsh{err: errors.New("MarshalBSONValue error")},
			bsonx.Arr{},
			errors.New("MarshalBSONValue error"),
		},
		{
			"bsoncodec.ValueMarshaler/not array",
			bvMarsh{t: bsontype.String},
			bsonx.Arr{},
			fmt.Errorf("ValueMarshaler returned a %v, but was expecting %v", bsontype.String, bsontype.Array),
		},
		{
			"bsoncodec.ValueMarshaler/UnmarshalBSONValue error",
			bvMarsh{t: bsontype.Array},
			bsonx.Arr{},
			bsoncore.NewInsufficientBytesError(nil, nil),
		},
		{
			"bsoncodec.ValueMarshaler/success",
			bvMarsh{t: bsontype.Array, data: arr},
			bsonx.Arr{bsonx.Document(bsonx.Doc{{"$limit", bsonx.Int32(12345)}})},
			nil,
		},
		{
			"nil",
			nil,
			bsonx.Arr{},
			errors.New("can only transform slices and arrays into aggregation pipelines, but got invalid"),
		},
		{
			"not array or slice",
			int64(42),
			bsonx.Arr{},
			errors.New("can only transform slices and arrays into aggregation pipelines, but got int64"),
		},
		{
			"array/error",
			[1]interface{}{int64(42)},
			bsonx.Arr{},
			MarshalError{Value: int64(0), Err: errors.New("WriteInt64 can only write while positioned on a Element or Value but is positioned on a TopLevel")},
		},
		{
			"array/success",
			[1]interface{}{primitive.D{{"$limit", int64(12345)}}},
			bsonx.Arr{bsonx.Document(bsonx.Doc{{"$limit", bsonx.Int64(12345)}})},
			nil,
		},
		{
			"slice/error",
			[]interface{}{int64(42)},
			bsonx.Arr{},
			MarshalError{Value: int64(0), Err: errors.New("WriteInt64 can only write while positioned on a Element or Value but is positioned on a TopLevel")},
		},
		{
			"slice/success",
			[]interface{}{primitive.D{{"$limit", int64(12345)}}},
			bsonx.Arr{bsonx.Document(bsonx.Doc{{"$limit", bsonx.Int64(12345)}})},
			nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			arr, err := transformAggregatePipeline(bson.NewRegistryBuilder().Build(), tc.pipeline)
			if !cmp.Equal(err, tc.err, cmp.Comparer(compareErrors)) {
				t.Errorf("Error does not match expected error. got %v; want %v", err, tc.err)
			}
			if !cmp.Equal(arr, tc.arr, cmp.AllowUnexported(bsonx.Val{})) {
				t.Errorf("Returned array does not match expected array. got %v; want %v", arr, tc.arr)
			}
		})
	}
}

func compareErrors(err1, err2 error) bool {
	if err1 == nil && err2 == nil {
		return true
	}

	if err1 == nil || err2 == nil {
		return false
	}

	if err1.Error() != err2.Error() {
		return false
	}

	return true
}

var _ bson.Marshaler = bMarsh{}

type bMarsh struct {
	bsonx.Doc
}

func (b bMarsh) MarshalBSON() ([]byte, error) {
	return b.Doc.MarshalBSON()
}

type reflectStruct struct {
	Foo string
}

var _ bsoncodec.ValueMarshaler = bvMarsh{}

type bvMarsh struct {
	t    bsontype.Type
	data []byte
	err  error
}

func (b bvMarsh) MarshalBSONValue() (bsontype.Type, []byte, error) {
	return b.t, b.data, b.err
}
