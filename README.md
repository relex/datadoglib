# datadoglib
Datadog simulation tool / mock

## Usage
```go run main.go```

### Flags:
- `port` - overrides the application default port (8083)
- `random_no_auth` - chance to mock a failed auth error and get a 403 response. float 0.0 to 1.0
- `random_slow_receive` - chance for a request to hang for a while before responding. float 0.0 to 1.0
- `random_bad_response` - chance to receive a status 500 response. float 0.0 to 1.0
- `random_network_lag` - maximum random network lag on receiving request and responding, msec
- `disable_json_parsing` - print out raw json payload without unmarshalling, bool
