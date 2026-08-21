package selfcheck

import "encoding/json"

// jsonDecode is a thin shim so scenario files that decode bodies don't each
// import encoding/json (keeps the import surface minimal in the scenario
// files).
func jsonDecode(body []byte, v any) error { return json.Unmarshal(body, v) }
