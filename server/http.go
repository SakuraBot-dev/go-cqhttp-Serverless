package server

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"time"

	"github.com/Mrs4s/go-cqhttp/coolq"

	"github.com/Mrs4s/MiraiGo/utils"
	"github.com/guonaihong/gout"
	"github.com/guonaihong/gout/dataflow"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
)

// HTTPClient 反向HTTP上报客户端
type HTTPClient struct {
	bot     *coolq.CQBot
	secret  string
	addr    string
	timeout int32
}

// Debug 是否启用Debug模式
var Debug = false

// NewHTTPClient 返回反向HTTP客户端
func NewHTTPClient() *HTTPClient {
	return &HTTPClient{}
}

// Run 运行反向HTTP服务
func (c *HTTPClient) Run(addr, secret string, timeout int32, bot *coolq.CQBot) {
	c.bot = bot
	c.secret = secret
	c.addr = addr
	c.timeout = timeout
	if c.timeout < 5 {
		c.timeout = 5
	}
	bot.OnEventPush(c.onBotPushEvent)
	log.Infof("HTTP POST上报器已启动: %v", addr)
}

func (c *HTTPClient) onBotPushEvent(m *bytes.Buffer) {
	var res string
	err := gout.POST(c.addr).SetJSON(m.Bytes()).BindBody(&res).SetHeader(func() gout.H {
		h := gout.H{
			"X-Self-ID":  c.bot.Client.Uin,
			"User-Agent": "CQHttp/4.15.0",
		}
		if c.secret != "" {
			mac := hmac.New(sha1.New, []byte(c.secret))
			_, err := mac.Write(m.Bytes())
			if err != nil {
				log.Error(err)
				return nil
			}
			h["X-Signature"] = "sha1=" + hex.EncodeToString(mac.Sum(nil))
		}
		return h
	}()).SetTimeout(time.Second * time.Duration(c.timeout)).F().Retry().Attempt(5).
		WaitTime(time.Millisecond * 500).MaxWaitTime(time.Second * 5).
		Func(func(con *dataflow.Context) error {
			if con.Error != nil {
				log.Warnf("上报Event到 HTTP 服务器 %v 时出现错误: %v 将重试.", c.addr, con.Error)
				return con.Error
			}
			return nil
		}).Do()
	if err != nil {
		log.Warnf("上报Event数据 %v 到 %v 失败: %v", utils.B2S(m.Bytes()), c.addr, err)
		return
	}
	log.Debugf("上报Event数据 %v 到 %v", utils.B2S(m.Bytes()), c.addr)
	if gjson.Valid(res) {
		c.bot.CQHandleQuickOperation(gjson.Parse(utils.B2S(m.Bytes())), gjson.Parse(res))
	}
}
