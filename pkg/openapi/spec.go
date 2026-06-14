package openapi

import (
	"encoding/json"
	"net/http"
)

type Spec struct {
	doc map[string]interface{}
}

func New(host string) *Spec {
	if host == "" {
		host = "localhost:8989"
	}
	return &Spec{doc: buildDoc(host)}
}

func (s *Spec) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(s.doc)
}

func buildDoc(host string) map[string]interface{} {
	return map[string]interface{}{
		"openapi": "3.0.3",
		"info": map[string]interface{}{
			"title":       "GridSim API",
			"version":     "3.1.0",
			"description": "IEC104/Modbus power grid simulator with multi-instance management, real-time point control, microgrid simulation, and API testing tools. Error responses: {\"error\":{\"code\":\"ERROR_CODE\",\"message\":\"...\",\"hint\":\"...\",\"candidates\":[...]}}",
		},
		"servers": []interface{}{
			map[string]interface{}{"url": "http://" + host},
		},
		"tags": []interface{}{
			map[string]interface{}{"name": "Auth"},
			map[string]interface{}{"name": "Instances"},
			map[string]interface{}{"name": "Points"},
			map[string]interface{}{"name": "Status"},
			map[string]interface{}{"name": "Files"},
			map[string]interface{}{"name": "Proxy"},
			map[string]interface{}{"name": "Microgrid"},
		},
		"paths":      buildPaths(),
		"components": buildComponents(),
	}
}

func buildPaths() map[string]interface{} {
	p := map[string]interface{}{}

	add := func(path string, methods map[string]interface{}) {
		p[path] = methods
	}

	add("/api/v1/auth/login", map[string]interface{}{
		"post": op("Auth", "Login", "Authenticate and get session cookie",
			nil, jsonBody(map[string]interface{}{
				"username": prop("string", "admin"),
				"password": prop("string", "password"),
			}), codes("200", "Login success", "401", "Invalid credentials")),
	})

	add("/api/v1/instances", map[string]interface{}{
		"get": op("Instances", "ListInstances", "List all configured instances", nil, nil, codes("200", "Array of instances")),
		"post": op("Instances", "CreateInstance", "Create a new instance configuration", nil,
			jsonBody(map[string]interface{}{
				"id":          prop("string", "substation-a"),
				"name":        prop("string", "220kV Substation A"),
				"config_file": prop("string", "point.xlsx"),
				"port":        prop("integer", 2404),
				"protocol":    prop("string", "iec104"),
			}), codes("201", "Instance created", "400", "Bad request", "409", "Already exists")),
	})

	add("/api/v1/instances/{id}", map[string]interface{}{
		"get":    op("Instances", "GetInstance", "Get instance details", allParams(pID("Instance ID")), nil, codes("200", "Instance details", "404", "Not found")),
		"put":    op("Instances", "UpdateInstance", "Update instance configuration", allParams(pID("Instance ID")), nil, codes("200", "Updated", "404", "Not found")),
		"delete": op("Instances", "DeleteInstance", "Delete instance", allParams(pID("Instance ID")), nil, codes("200", "Deleted", "404", "Not found")),
	})

	add("/api/v1/instances/{id}/start", map[string]interface{}{
		"post": op("Instances", "StartInstance", "Start instance (begins listening on its port)", allParams(pID("Instance ID")), nil, codes("200", "Started", "404", "Not found", "409", "Port in use")),
	})
	add("/api/v1/instances/{id}/stop", map[string]interface{}{
		"post": op("Instances", "StopInstance", "Stop instance", allParams(pID("Instance ID")), nil, codes("200", "Stopped", "404", "Not found")),
	})
	add("/api/v1/instances/{id}/restart", map[string]interface{}{
		"post": op("Instances", "RestartInstance", "Restart instance", allParams(pID("Instance ID")), nil, codes("200", "Restarted", "404", "Not found")),
	})

	add("/api/v1/instances/{id}/points", map[string]interface{}{
		"get": op("Points", "ListPoints", "Get all point values (real-time snapshot)", allParams(pID("Instance ID")), nil, codes("200", "Array of points", "404", "Not found")),
	})
	add("/api/v1/instances/{id}/points/batch", map[string]interface{}{
		"get": op("Points", "BatchGetPoints", "Batch read points by IOA list",
			allParams(pID("Instance ID"), queryParam("ioas", "Comma-separated IOA numbers (e.g. 1,2,16385)", true)),
			nil, codes("200", "Array of point snapshots", "400", "Bad request")),
	})
	add("/api/v1/instances/{id}/points/{ioa}", map[string]interface{}{
		"get": op("Points", "GetPoint", "Get single point value", allParams(pID("Instance ID"), pIOA()), nil, codes("200", "Point snapshot", "404", "Not found")),
		"put": op("Points", "SetPoint", "Set point value (triggers spontaneous transmission COT=3)", allParams(pID("Instance ID"), pIOA()),
			jsonBody(map[string]interface{}{"value": prop("number", 235.5), "bool_value": prop("boolean", true)}),
			codes("200", "Updated", "400", "Bad request", "404", "Not found")),
	})

	add("/api/v1/instances/{id}/points/auto-change/{ioa}", map[string]interface{}{
		"get":    op("Points", "GetAutoChange", "Get auto-change strategy config", allParams(pID("Instance ID"), pIOA()), nil, codes("200", "Strategy config", "404", "Not found")),
		"put":    op("Points", "SetAutoChange", "Configure auto-change strategy", allParams(pID("Instance ID"), pIOA()), nil, codes("200", "Configured", "400", "Bad request")),
		"delete": op("Points", "DeleteAutoChange", "Remove auto-change strategy", allParams(pID("Instance ID"), pIOA()), nil, codes("200", "Deleted", "404", "Not found")),
	})
	add("/api/v1/instances/{id}/points/auto-change/batch", map[string]interface{}{
		"put": op("Points", "BatchAutoChange", "Batch configure auto-change for multiple points", allParams(pID("Instance ID")), nil, codes("200", "Batch result", "400", "Bad request")),
	})
	add("/api/v1/instances/{id}/points/auto-change/export", map[string]interface{}{
		"get": op("Points", "ExportAutoChange", "Export all auto-change configs as JSON", allParams(pID("Instance ID")), nil, codes("200", "Export data")),
	})
	add("/api/v1/instances/{id}/points/auto-change/import", map[string]interface{}{
		"post": op("Points", "ImportAutoChange", "Import auto-change configs from JSON", allParams(pID("Instance ID")), nil, codes("200", "Import result", "400", "Bad request")),
	})
	add("/api/v1/instances/{id}/points/export", map[string]interface{}{
		"get": op("Points", "ExportPointsCSV", "Export all point values as CSV", allParams(pID("Instance ID")), nil, codes("200", "CSV data")),
	})

	add("/api/v1/status", map[string]interface{}{
		"get": op("Status", "GetStatus", "Get global service status (uptime, instance counts, max instances)", nil, nil, codes("200", "Service status")),
	})

	add("/api/v1/upload", map[string]interface{}{
		"post": op("Files", "UploadConfig", "Upload .xlsx point configuration file (multipart/form-data)", nil, nil, codes("200", "Upload result", "400", "Bad request")),
	})
	add("/api/v1/files", map[string]interface{}{
		"get": op("Files", "ListFiles", "List uploaded config files", nil, nil, codes("200", "File list")),
	})
	add("/api/v1/protocols", map[string]interface{}{
		"get": op("Files", "ListProtocols", "List supported protocols (iec104, modbus)", nil, nil, codes("200", "Protocol list")),
	})

	add("/api/v1/proxy", map[string]interface{}{
		"post": op("Proxy", "ExecuteProxy", "Execute HTTP request through proxy (supports pre/post scripts)", nil,
			jsonBody(map[string]interface{}{"method": prop("string", "GET"), "url": prop("string", "http://example.com/api")}),
			codes("200", "Proxy response", "400", "Bad request")),
	})
	add("/api/v1/proxy/collections", map[string]interface{}{
		"get":  op("Proxy", "ListCollections", "List API test collections", nil, nil, codes("200", "Collection list")),
		"post": op("Proxy", "UpsertCollection", "Create or update a collection item", nil, jsonBody(map[string]interface{}{"name": prop("string", "My Collection"), "method": prop("string", "GET"), "url": prop("string", "http://...")}), codes("200", "Upserted", "400", "Bad request")),
	})
	add("/api/v1/proxy/collections/{id}", map[string]interface{}{
		"delete": op("Proxy", "DeleteCollection", "Delete a collection item", allParams(pID("Collection item ID")), nil, codes("200", "Deleted", "404", "Not found")),
	})
	add("/api/v1/proxy/environments", map[string]interface{}{
		"get":  op("Proxy", "ListEnvironments", "List proxy environments", nil, nil, codes("200", "Environment list")),
		"post": op("Proxy", "UpsertEnvironment", "Create or update an environment", nil, jsonBody(map[string]interface{}{"name": prop("string", "Production")}), codes("200", "Upserted", "400", "Bad request")),
	})
	add("/api/v1/proxy/environments/{id}/activate", map[string]interface{}{
		"post": op("Proxy", "ActivateEnvironment", "Activate an environment", allParams(pID("Environment ID")), nil, codes("200", "Activated", "404", "Not found")),
	})
	add("/api/v1/proxy/environments/{id}", map[string]interface{}{
		"delete": op("Proxy", "DeleteEnvironment", "Delete an environment", allParams(pID("Environment ID")), nil, codes("200", "Deleted", "404", "Not found")),
	})

	add("/api/v1/microgrid/topology", map[string]interface{}{
		"get":  op("Microgrid", "GetTopology", "Get microgrid topology (devices, connections)", nil, nil, codes("200", "Topology object")),
		"post": op("Microgrid", "SaveTopology", "Save microgrid topology", nil, jsonBody(map[string]interface{}{"devices": map[string]interface{}{"type": "array"}, "connections": map[string]interface{}{"type": "array"}}), codes("200", "Saved", "400", "Bad request")),
	})
	add("/api/v1/microgrid/control", map[string]interface{}{
		"post": op("Microgrid", "MicrogridControl", "Control a microgrid device (switch on/off, set power)", nil,
			jsonBody(map[string]interface{}{"device_id": prop("string", "pv-1"), "action": prop("string", "on")}),
			codes("200", "Control result", "400", "Bad request")),
	})
	add("/api/v1/microgrid/dashboard", map[string]interface{}{
		"get": op("Microgrid", "MicrogridDashboard", "Get microgrid dashboard (real-time power flow, SOC)", nil, nil, codes("200", "Dashboard data")),
	})
	add("/api/v1/microgrid/formulas", map[string]interface{}{
		"get":  op("Microgrid", "ListFormulas", "List custom formulas", nil, nil, codes("200", "Formula list")),
		"post": op("Microgrid", "SaveFormula", "Create or update a custom formula", nil, jsonBody(map[string]interface{}{"name": prop("string", "total_power"), "expression": prop("string", "{0} + {1}")}), codes("200", "Saved", "400", "Bad request")),
	})
	add("/api/v1/microgrid/export-xlsx", map[string]interface{}{
		"get": op("Microgrid", "ExportMicrogridXLSX", "Export microgrid point table as .xlsx", nil, nil, codes("200", "XLSX file download")),
	})
	add("/api/v1/microgrid/points", map[string]interface{}{
		"get": op("Microgrid", "MicrogridPoints", "Get microgrid point values", nil, nil, codes("200", "Point values")),
	})

	return p
}

func buildComponents() map[string]interface{} {
	return map[string]interface{}{
		"schemas": map[string]interface{}{
			"Error": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"error": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"code":       map[string]interface{}{"type": "string", "example": "NOT_FOUND"},
							"message":    map[string]interface{}{"type": "string", "example": "instance not found"},
							"hint":       map[string]interface{}{"type": "string", "example": "Use GET /api/v1/instances to list available instances"},
							"candidates": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "example": []string{"substation-a", "substation-b"}},
							"field":      map[string]interface{}{"type": "string", "example": "id"},
						},
						"required": []string{"code", "message"},
					},
				},
			},
			"Point": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"ioa":        map[string]interface{}{"type": "integer", "example": 16385},
					"name":       map[string]interface{}{"type": "string", "example": "Bus Voltage"},
					"point_type": map[string]interface{}{"type": "string", "enum": []string{"AI", "DI", "PI", "DO", "AO"}},
					"value":      map[string]interface{}{"type": "number", "example": 220.5},
					"quality":    map[string]interface{}{"type": "integer", "example": 0},
					"updated_at": map[string]interface{}{"type": "string", "format": "date-time"},
				},
			},
			"Instance": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id":          map[string]interface{}{"type": "string"},
					"name":        map[string]interface{}{"type": "string"},
					"port":        map[string]interface{}{"type": "integer"},
					"protocol":    map[string]interface{}{"type": "string", "enum": []string{"iec104", "modbus"}},
					"status":      map[string]interface{}{"type": "string", "enum": []string{"running", "stopped", "error"}},
					"config_file": map[string]interface{}{"type": "string"},
					"point_count": map[string]interface{}{"type": "integer"},
				},
			},
		},
	}
}

func op(tag, summary, description string, parameters interface{}, reqBody interface{}, responses map[string]interface{}) map[string]interface{} {
	m := map[string]interface{}{
		"tags":        []string{tag},
		"summary":     summary,
		"description": description,
		"responses":   responses,
	}
	if parameters != nil {
		m["parameters"] = parameters
	}
	if reqBody != nil {
		m["requestBody"] = reqBody
	}
	return m
}

func jsonBody(props map[string]interface{}) interface{} {
	return map[string]interface{}{
		"required": true,
		"content": map[string]interface{}{
			"application/json": map[string]interface{}{
				"schema": map[string]interface{}{"type": "object", "properties": props},
			},
		},
	}
}

func codes(codeDesc ...string) map[string]interface{} {
	m := map[string]interface{}{}
	for i := 0; i < len(codeDesc); i += 2 {
		code := codeDesc[i]
		desc := codeDesc[i+1]
		if code == "200" || code == "201" {
			m[code] = map[string]interface{}{"description": desc}
		} else {
			m[code] = map[string]interface{}{
				"description": desc,
				"content": map[string]interface{}{
					"application/json": map[string]interface{}{
						"schema": map[string]interface{}{"$ref": "#/components/schemas/Error"},
					},
				},
			}
		}
	}
	return m
}

func allParams(ps ...interface{}) interface{} { return ps }

func pID(desc string) map[string]interface{} {
	return map[string]interface{}{"name": "id", "in": "path", "required": true, "description": desc, "schema": map[string]interface{}{"type": "string"}}
}

func pIOA() map[string]interface{} {
	return map[string]interface{}{"name": "ioa", "in": "path", "required": true, "description": "Information Object Address", "schema": map[string]interface{}{"type": "integer"}}
}

func queryParam(name, desc string, required bool) map[string]interface{} {
	return map[string]interface{}{"name": name, "in": "query", "required": required, "description": desc, "schema": map[string]interface{}{"type": "string"}}
}

func prop(typ string, exampleVal interface{}) map[string]interface{} {
	return map[string]interface{}{"type": typ, "example": exampleVal}
}
