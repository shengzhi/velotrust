package codec

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	jsonrpc "velotrust/pkg/jrpc/error"
)

var null = json.RawMessage([]byte("null"))

// ----------------------------------------------------------------------------
// Request and Response
// ----------------------------------------------------------------------------

// serverRequest represents a JSON-RPC request received by the server.
type serverRequest struct {
	// A String containing the name of the method to be invoked.
	Method string `json:"method"`
	// An Array of objects to pass as arguments to the method.
	Params *json.RawMessage `json:"params"`
	// The request id. This can be of any type. It is used to match the
	// response with the request that it is replying to.
	Id string `json:"id"`
}

// serverResponse represents a JSON-RPC response returned by the server.
type serverResponse struct {
	// The Object that was returned by the invoked method. This must be null
	// in case there was an error invoking the method.
	Result interface{} `json:"result"`
	// An Error object if there was an error invoking the method. It must be
	// null if there was no error.
	Error interface{} `json:"error"`
	// Code 200 is correct result, others are error result.
	Code int `json:"code"`
	// This must be the same id as the request it is responding to.
	Id string `json:"id"`
}

type JSONCodec struct {
}

func (jc JSONCodec) NewRequest(r *http.Request) CodecRequest {
	defer r.Body.Close()
	var req serverRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	return &jsonCodecRequest{req: req, decodeErr: err}
}

type jsonCodecRequest struct {
	req       serverRequest
	decodeErr error
	err       error
}

func (jcr *jsonCodecRequest) Method() (string, error) {
	if jcr.decodeErr != nil {
		return "", jcr.decodeErr
	}
	if jcr.req.Method == "" {
		return "", fmt.Errorf("invalid method param")
	}
	return jcr.req.Method, nil
}

func (jcr *jsonCodecRequest) RequestID() string {
	return jcr.req.Id
}

func (jcr *jsonCodecRequest) Params() []byte {
	if jcr.req.Params == nil {
		return nil
	}
	return *jcr.req.Params
}

func (jcr *jsonCodecRequest) ReadRequest(args interface{}) error {
	if jcr.decodeErr != nil {
		return jcr.decodeErr
	}

	if jcr.req.Params != nil {
		// JSON params is array value. RPC params is struct.
		// Unmarshal into array containing the request struct.
		if isArrayParam(*jcr.req.Params) {
			params := [1]interface{}{args}
			jcr.err = json.Unmarshal(*jcr.req.Params, &params)
		} else {
			jcr.err = json.Unmarshal(*jcr.req.Params, args)
		}
	} else {
		jcr.err = errors.New("rpc: method request ill-formed: missing params field")
	}
	return jcr.err
}

// WriteResponse encodes the response and writes it to the ResponseWriter.
//
// The err parameter is the error resulted from calling the RPC method,
// or nil if there was no error.
func (jcr *jsonCodecRequest) WriteResponse(w http.ResponseWriter, reply interface{}, methodErr error) error {
	jcr.err = WriteJSONResponse(w, jcr.req.Id, reply, methodErr)
	return jcr.err
}

func WriteJSONResponse(w http.ResponseWriter, requestID string, reply interface{}, methodErr error) error {
	res := &serverResponse{
		Result: reply,
		Error:  &null,
		Code:   http.StatusOK,
		Id:     requestID,
	}
	if methodErr != nil {
		rpcErr := normalizeError(methodErr)
		res.Code = rpcErr.ErrCode()
		res.Error = rpcErr
		res.Result = &null
	}
	if requestID == "" && methodErr == nil {
		return nil
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	return json.NewEncoder(w).Encode(res)
}

func normalizeError(err error) jsonrpc.RpcError {
	if err == nil {
		return nil
	}
	var rpcErr jsonrpc.RpcError
	if errors.As(err, &rpcErr) {
		value, ok := rpcErr.(*jsonrpc.Error)
		if ok && value == nil {
			return jsonrpc.ErrInternal()
		}
		if ok && value.Data != nil && len(value.Data.Args) > 0 {
			if _, marshalErr := json.Marshal(value.Data.Args); marshalErr != nil {
				data := *value.Data
				data.Args = nil
				copy := *value
				copy.Data = &data
				return &copy
			}
		}
		return rpcErr
	}
	return &jsonrpc.Error{
		Code:    jsonrpc.ErrorCodeInternal,
		Message: "Internal error",
	}
}

func isArrayParam(p json.RawMessage) bool {
	if len(p) <= 0 {
		return false
	}
	return p[0] == '['
}
