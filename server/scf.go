package server

import (
	"context"
	"strings"

	jsoniter "github.com/json-iterator/go"
	log "github.com/sirupsen/logrus"
	"github.com/tencentyun/scf-go-lib/events"
	"github.com/tencentyun/scf-go-lib/functioncontext"
	"github.com/tidwall/gjson"

	"github.com/Mrs4s/go-cqhttp/coolq"
)

// SCFEvent 云函数Event结构体
type SCFEvent struct {
	Type        string                       `json:"type"`
	Method      string                       `json:"httpMethod"`
	Path        string                       `json:"path"`
	QueryString events.APIGatewayQueryString `json:"queryString"`
	Body        string                       `json:"body"`
	Headers     map[string]string            `json:"headers"`
}

type scfServer struct {
	api       *apiCaller
	authToken string
}

var json = jsoniter.ConfigCompatibleWithStandardLibrary

// SCFServer SCF服务器
var SCFServer = &scfServer{
	api: &apiCaller{},
}

// IsUp GoCQ是否已启动
var IsUp = false

func (s *scfServer) Run(authToken string, bot *coolq.CQBot) {
	s.api.bot = bot
	s.authToken = authToken
	IsUp = true
}

// SCFHandler 云函数回调
func SCFHandler(ctx context.Context, event SCFEvent) (data *APIGatewayResponse, err error) {
	lc, _ := functioncontext.FromContext(ctx)
	res := coolq.MSG{}
	switch IsUp {
	case false:
		log.Debugf("go-cqhttp-Serverless接收到Api调用，正在启动中")
		res = coolq.OK(coolq.MSG{
			"SCFStatus": "Starting",
		})
	default:
		if event.Type == "Timer" {
			log.Debugf("go-cqhttp-Serverless接收到定时器调用")
			res = coolq.OK(coolq.MSG{"SCFStatus": "OKTimer"})
		} else {
			authToken := SCFServer.authToken
			if authToken != "" {
				if auth := event.Headers["authorization"]; auth != "" {
					if strings.SplitN(auth, " ", 2)[1] != authToken {
						res["status"] = "Unauthorized"
						return GatewayResponse(401, res), nil
					}
				}
				if event.QueryString["access_token"] == nil || event.QueryString["access_token"][0] != authToken {
					res["status"] = "Unauthorized"
					return GatewayResponse(401, res), nil
				}
				action := strings.ReplaceAll(event.Path, "_async", "")
				log.Debugf("SCFServer接收到API调用: %v", action)
				action = strings.Replace(event.Path, "/", "", 1)
				res = SCFServer.api.callAPI(action, event)
			}
		}
	}
	if Debug {
		res["raw"] = event
	}
	res["request_id"] = lc.RequestID
	return GatewayResponse(200, res), nil
}

func (e SCFEvent) Get(k string) gjson.Result {
	if q := e.QueryString[k]; q != nil {
		return gjson.Result{Type: gjson.String, Str: q[0]}
	}
	if e.Method == "POST" {
		if strings.Contains(e.Headers["content-type"], "application/x-www-form-urlencoded") {
			return gjson.Result{Type: gjson.String, Str: e.Body}
		}
		if strings.Contains(e.Headers["content-type"], "application/json") {
			return gjson.Get(e.Body, k)
		}
	}
	return gjson.Result{Type: gjson.Null, Str: ""}
}

// APIGateWwayResponse API网关集成响应结构体
type APIGatewayResponse struct {
	IsBase64Encoded bool `json:"isBase64Encoded"`
	StatusCode      int  `json:"statusCode"`
	Headers         struct {
		ContentType string `json:"Content-Type"`
	} `json:"headers"`
	Body string `json:"body"`
}

// GatewayResponse 构建API网关集成响应结构体
func GatewayResponse(code int, data interface{}) *APIGatewayResponse {
	dataByte, err := json.Marshal(data)
	if err != nil {
		log.Error(err.Error())
	}
	res := &APIGatewayResponse{
		IsBase64Encoded: false,
		StatusCode:      code,
		Headers: struct {
			ContentType string "json:\"Content-Type\""
		}{
			ContentType: "application/json; charset=utf-8",
		},
		Body: string(dataByte),
	}
	return res
}
