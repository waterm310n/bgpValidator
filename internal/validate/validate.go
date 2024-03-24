// 使用routinator进行起源AS与IP前缀的合法验证
package validate

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
)

type RoutinatorValidator struct {
	url url.URL
}

type ValidatedMessage struct {
	ValidatedRoute *ValidatedRoute `json:"validated_route,omitempty"`
}

type ValidatedRoute struct {
	Validity *Validity `json:"validity,omitempty"`
}

type Validity struct {
	State string `json:"state,omitempty"`
}

func NewRoutinatorValidator(scheme, host, path string) *RoutinatorValidator {
	url := url.URL{
		Scheme: scheme,
		Host:   host,
		Path:   path,
	}
	return &RoutinatorValidator{
		url,
	}
}

// 验证起源AS是否与前缀prefiex对应
func (r *RoutinatorValidator) Validate(originASN string, prefix string) bool {
	values := url.Values{}
	values.Add("asn", originASN)
	values.Add("prefix", prefix)
	r.url.RawQuery = values.Encode()
	if resp, err := http.Get(r.url.String()); err == nil {
		//TODO io.ReadAll每次会扩容两次，可能影响效率，换成1024字节的初始byte数组会更好
		if buf, err := io.ReadAll(resp.Body); err == io.EOF ||err == nil {
			var validatedMessage ValidatedMessage
			if err := json.Unmarshal(buf, &validatedMessage);  err == nil  {
				switch validatedMessage.ValidatedRoute.Validity.State {
				case "invalid":
					return false
				default:
					return true
				}
			} else {
				slog.Error(err.Error())
			}
		} else {
			slog.Error(err.Error())
		}
	} else {
		slog.Error(err.Error())
	}

	return true
}
