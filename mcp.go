package main

import (
	"bufio"
	"encoding/json"
	"os"
)

type rpcMsg struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}

func runMCP() {
	in := bufio.NewReaderSize(os.Stdin, 1<<20)
	enc := json.NewEncoder(os.Stdout)
	reply := func(id json.RawMessage, result interface{}, rpcErr interface{}) {
		m := map[string]interface{}{"jsonrpc": "2.0", "id": id}
		if rpcErr != nil {
			m["error"] = rpcErr
		} else {
			m["result"] = result
		}
		enc.Encode(m)
	}
	for {
		line, err := in.ReadBytes('\n')
		if len(line) > 1 {
			var m rpcMsg
			if json.Unmarshal(line, &m) != nil {
				continue
			}
			if len(m.ID) == 0 {
				continue
			}
			switch m.Method {
			case "initialize":
				var p struct {
					ProtocolVersion string `json:"protocolVersion"`
				}
				json.Unmarshal(m.Params, &p)
				if p.ProtocolVersion == "" {
					p.ProtocolVersion = "2024-11-05"
				}
				reply(m.ID, map[string]interface{}{
					"protocolVersion": p.ProtocolVersion,
					"capabilities":    map[string]interface{}{"tools": map[string]interface{}{}},
					"serverInfo":      map[string]string{"name": "replado-connector", "version": version},
				}, nil)
			case "ping":
				reply(m.ID, map[string]interface{}{}, nil)
			case "tools/list":
				reply(m.ID, map[string]interface{}{"tools": toolDefs}, nil)
			case "tools/call":
				var p struct {
					Name      string          `json:"name"`
					Arguments json.RawMessage `json:"arguments"`
				}
				json.Unmarshal(m.Params, &p)
				res, err := callTool(p.Name, p.Arguments)
				var text string
				isErr := false
				if err != nil {
					text, isErr = err.Error(), true
				} else {
					b, _ := json.Marshal(res)
					text = string(b)
				}
				reply(m.ID, map[string]interface{}{
					"content": []map[string]string{{"type": "text", "text": text}},
					"isError": isErr,
				}, nil)
			default:
				reply(m.ID, nil, map[string]interface{}{"code": -32601, "message": "Methode unbekannt"})
			}
		}
		if err != nil {
			return
		}
	}
}
