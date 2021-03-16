package server

import (
	"context"
	"strings"

	"github.com/Mrs4s/MiraiGo/client"
	log "github.com/sirupsen/logrus"
	"github.com/tencentyun/scf-go-lib/events"
	"github.com/tencentyun/scf-go-lib/functioncontext"
	"github.com/tidwall/gjson"

	"github.com/Mrs4s/go-cqhttp/coolq"
)

// SCFEvent 云函数Event结构体
type SCFEvent struct {
	ContentType string                          `json:"content-type"`
	RequestCtx  events.APIGatewayRequestContext `json:"requestContext"`
	Method      string                          `json:"httpMethod"`
	Path        string                          `json:"path"`
	QueryString events.APIGatewayQueryString    `json:"queryString"`
	Body        string                          `json:"body"`
	Headers     map[string]string               `json:"headers"`
}

type scfServer struct {
	api *apiCaller
}

// SCFServer SCF服务器
var SCFServer = &scfServer{
	api: &apiCaller{
		bot: &coolq.CQBot{},
	},
}

type scfEntry struct {
	apiAdmin *webServer
}

// SCFEntry SCF入口点
var SCFEntry = &scfEntry{
	apiAdmin: &webServer{},
}

// IsUp GoCQ是否已启动
var IsUp = false

func (s *scfEntry) Run(cli *client.QQClient) *coolq.CQBot {
	s.apiAdmin.Cli = cli
	s.apiAdmin.Conf = GetConf()
	JSONConfig = s.apiAdmin.Conf
	s.apiAdmin.Dologin()
	b := s.apiAdmin.bot // 外部引入 bot对象，用于操作bot
	s.UpServer(b)
	return b
}

func (s *scfEntry) UpServer(b *coolq.CQBot) {
	conf := s.apiAdmin.Conf
	for k, v := range conf.HTTPConfig.PostUrls {
		newHTTPClient().Run(k, v, conf.HTTPConfig.Timeout, s.apiAdmin.bot)
	}
	for _, rc := range conf.ReverseServers {
		go NewWebSocketClient(rc, conf.AccessToken, s.apiAdmin.bot).Run()
	}
	SCFServer.api.bot = b
	IsUp = true
}

// SCFHandler 云函数回调
func SCFHandler(ctx context.Context, event SCFEvent) (data *APIGatewayReponse, err error) {
	lc, _ := functioncontext.FromContext(ctx)
	res := coolq.MSG{}
	if !IsUp {
		log.Debugf("go-cqhttp-Serverless接收到Api调用，正在启动中")
		res = coolq.OK(coolq.MSG{
			"SCFStatus": "Starting",
		})
	} else {
		authToken := SCFEntry.apiAdmin.Conf.AccessToken
		if authToken != "" {
			if auth := event.Headers["Authorization"]; auth != "" {
				if strings.SplitN(auth, " ", 2)[1] != authToken {
					return FailedGateway(401, "Unauthorized"), nil
				}
			} else if event.QueryString["access_token"] == nil || event.QueryString["access_token"][0] != authToken {
				return FailedGateway(401, "Unauthorized"), nil
			}
			action := strings.ReplaceAll(event.Path, "_async", "")
			log.Debugf("SCFServer接收到API调用: %v", action)
			action = strings.Replace(event.Path, "/", "", 1)
			res = SCFServer.api.callAPI(action, event)
			res["request_id"] = lc.RequestID
		}
	}
	return OKGateway(res), nil
}

func (e SCFEvent) Get(k string) gjson.Result {
	if q := e.QueryString[k]; q != nil {
		return gjson.Result{Type: gjson.String, Str: q[0]}
	}
	// TODO: POST不太好使，debug中
	if e.Method == "POST" {
		if strings.Contains(e.ContentType, "application/x-www-form-urlencoded") {
			return gjson.Result{Type: gjson.String, Str: e.Body}
		}
		if strings.Contains(e.ContentType, "application/json") {
			return gjson.Get(strings.ReplaceAll(e.Body, "\\", ""), k)
		}
	}
	return gjson.Result{Type: gjson.Null, Str: ""}
}

// APIGateWwayReponse API网关集成响应结构体
type APIGatewayReponse struct {
	IsBase64Encoded bool `json:"isBase64Encoded"`
	StatusCode      int  `json:"statusCode"`
	Headers         struct {
		ContentType string `json:"Content-Type"`
	} `json:"headers"`
	Body string `json:"body"`
}

// OKGateway API网关集成响应OK
func OKGateway(data interface{}) *APIGatewayReponse {
	dataByte, err := json.Marshal(data)
	if err != nil {
		log.Error(err.Error())
	}
	res := &APIGatewayReponse{
		IsBase64Encoded: false,
		StatusCode:      200,
		Headers: struct {
			ContentType string "json:\"Content-Type\""
		}{
			ContentType: "application/json; charset=utf-8",
		},
		Body: string(dataByte),
	}
	return res
}

// FailedGateway API网关集成响应Fail
func FailedGateway(code int, msg ...string) *APIGatewayReponse {
	res := &APIGatewayReponse{
		IsBase64Encoded: false,
		StatusCode:      code,
		Headers: struct {
			ContentType string "json:\"Content-Type\""
		}{
			ContentType: "application/json; charset=utf-8",
		},
		Body: strings.Join(msg, " "),
	}
	return res
}
