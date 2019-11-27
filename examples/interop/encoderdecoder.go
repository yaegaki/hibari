package main

import (
	"fmt"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/vmihailenco/msgpack"
	"github.com/yaegaki/hibari/examples/interop/pb"
)

type grpcEncoderDecoder struct {
}

type websocketEncoderDecoder struct {
}

func (grpcEncoderDecoder) EncodeAnyMessageBody(body interface{}) (interface{}, error) {
	any, ok := body.(*any.Any)
	if !ok {
		return nil, fmt.Errorf("Invalid body type")
	}

	var pbBody pb.MessageBody
	err := ptypes.UnmarshalAny(any, &pbBody)
	if err != nil {
		return nil, err
	}

	return pbBody.JSON, err
}

func (grpcEncoderDecoder) DecodeAnyMessageBody(body interface{}) (interface{}, error) {
	json, ok := body.(string)
	if !ok {
		return nil, fmt.Errorf("Invalid body type")
	}

	pbBody := pb.MessageBody{
		JSON: json,
	}

	return ptypes.MarshalAny(&pbBody)
}

func (websocketEncoderDecoder) EncodeAnyMessageBody(body interface{}) (interface{}, error) {
	bin, ok := body.([]byte)
	if !ok {
		return nil, fmt.Errorf("Invalid body type")
	}

	var json string
	err := msgpack.Unmarshal(bin, &json)
	return json, err
}

func (websocketEncoderDecoder) DecodeAnyMessageBody(body interface{}) (interface{}, error) {
	return msgpack.Marshal(body)
}
