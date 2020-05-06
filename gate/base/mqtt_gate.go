// Copyright 2014 mqant Author. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package basegate

import (
	"fmt"
	"reflect"
	"time"

	"github.com/liangdas/mqant/conf"
	"github.com/liangdas/mqant/gate"
	"github.com/liangdas/mqant/log"
	"github.com/liangdas/mqant/module"
	"github.com/liangdas/mqant/module/base"
	"github.com/liangdas/mqant/network"
)

var RPC_PARAM_SESSION_TYPE = gate.RPC_PARAM_SESSION_TYPE
var RPC_PARAM_ProtocolMarshal_TYPE = gate.RPC_PARAM_ProtocolMarshal_TYPE

type Gate struct {
	//module.RPCSerialize
	basemodule.BaseModule
	opts gate.Options
	judgeGuest func(session gate.Session) bool

	createAgent func() gate.Agent
}

func (this *Gate) defaultCreateAgentd() gate.Agent {
	a := NewMqttAgent(this.GetModule())
	return a
}

func (this *Gate) SetJudgeGuest(judgeGuest func(session gate.Session) bool) error {
	this.judgeGuest = judgeGuest
	return nil
}

/**
设置Session信息持久化接口
*/
func (this *Gate) SetRouteHandler(router gate.RouteHandler) error {
	this.opts.RouteHandler = router
	return nil
}

/**
设置Session信息持久化接口
*/
func (this *Gate) SetStorageHandler(storage gate.StorageHandler) error {
	this.opts.StorageHandler = storage
	return nil
}

/**
设置客户端连接和断开的监听器
*/
func (this *Gate) SetSessionLearner(sessionLearner gate.SessionLearner) error {
	this.opts.SessionLearner = sessionLearner
	return nil
}

/**
设置创建客户端Agent的函数
*/
func (this *Gate) SetCreateAgent(cfunc func() gate.Agent) error {
	this.createAgent = cfunc
	return nil
}
func (this *Gate) Options() gate.Options {
	return this.opts
}
func (this *Gate) GetStorageHandler() (storage gate.StorageHandler) {
	return this.opts.StorageHandler
}
func (this *Gate) GetGateHandler() gate.GateHandler {
	return this.opts.GateHandler
}
func (this *Gate) GetAgentLearner() gate.AgentLearner {
	return this.opts.AgentLearner
}
func (this *Gate) GetSessionLearner() gate.SessionLearner {
	return this.opts.SessionLearner
}
func (this *Gate) GetRouteHandler() gate.RouteHandler {
	return this.opts.RouteHandler
}
func (this *Gate) GetJudgeGuest() func(session gate.Session) bool {
	return this.judgeGuest
}
func (this *Gate) GetModule() module.RPCModule {
	return this.GetSubclass()
}

func (this *Gate) NewSession(data []byte) (gate.Session, error) {
	return NewSession(this.App, data)
}
func (this *Gate) NewSessionByMap(data map[string]interface{}) (gate.Session, error) {
	return NewSessionByMap(this.App, data)
}

func (this *Gate) OnConfChanged(settings *conf.ModuleSettings) {

}

/**
自定义rpc参数序列化反序列化  Session
*/
func (this *Gate) Serialize(param interface{}) (ptype string, p []byte, err error) {
	rv := reflect.ValueOf(param)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		//不是指针
		return "", nil, fmt.Errorf("Serialize [%v ] or not pointer type", rv.Type())
	}
	switch v2 := param.(type) {
	case gate.Session:
		bytes, err := v2.Serializable()
		if err != nil {
			return RPC_PARAM_SESSION_TYPE, nil, err
		}
		return RPC_PARAM_SESSION_TYPE, bytes, nil
	case module.ProtocolMarshal:
		bytes := v2.GetData()
		return RPC_PARAM_ProtocolMarshal_TYPE, bytes, nil
	default:
		return "", nil, fmt.Errorf("args [%s] Types not allowed", reflect.TypeOf(param))
	}
}

func (this *Gate) Deserialize(ptype string, b []byte) (param interface{}, err error) {
	switch ptype {
	case RPC_PARAM_SESSION_TYPE:
		mps, errs := NewSession(this.App, b)
		if errs != nil {
			return nil, errs
		}
		return mps.Clone(), nil
	case RPC_PARAM_ProtocolMarshal_TYPE:
		return this.App.NewProtocolMarshal(b), nil
	default:
		return nil, fmt.Errorf("args [%s] Types not allowed", ptype)
	}
}

func (this *Gate) GetTypes() []string {
	return []string{RPC_PARAM_SESSION_TYPE}
}
func (this *Gate) OnAppConfigurationLoaded(app module.App) {
	//添加Session结构体的序列化操作类
	this.BaseModule.OnAppConfigurationLoaded(app) //这是必须的
	err := app.AddRPCSerialize("gate", this)
	if err != nil {
		log.Warning("Adding session structures failed to serialize interfaces %s", err.Error())
	}
}
func (this *Gate) OnInit(subclass module.RPCModule, app module.App, settings *conf.ModuleSettings,opts ...gate.Option) {
	this.opts = gate.NewOptions(opts...)
	this.BaseModule.OnInit(subclass, app, settings,this.opts.Opts...) //这是必须的
	if this.opts.WsAddr==""{
		if WSAddr, ok := settings.Settings["WSAddr"]; ok {
			this.opts.WsAddr = WSAddr.(string)
		}
	}
	if this.opts.TcpAddr==""{
		if TCPAddr, ok := settings.Settings["TCPAddr"]; ok {
			this.opts.TcpAddr = TCPAddr.(string)
		}
	}

	if this.opts.Tls==false{
		if Tls, ok := settings.Settings["Tls"]; ok {
			this.opts.Tls = Tls.(bool)
		} else {
			this.opts.Tls = false
		}
	}

	if this.opts.CertFile==""{
		if CertFile, ok := settings.Settings["CertFile"]; ok {
			this.opts.CertFile = CertFile.(string)
		} else {
			this.opts.CertFile = ""
		}
	}

	if this.opts.KeyFile==""{
		if KeyFile, ok := settings.Settings["KeyFile"]; ok {
			this.opts.KeyFile = KeyFile.(string)
		} else {
			this.opts.KeyFile = ""
		}
	}

	handler := NewGateHandler(this)

	this.opts.AgentLearner = handler
	this.opts.GateHandler = handler
	this.GetServer().RegisterGO("Update", this.opts.GateHandler.Update)
	this.GetServer().RegisterGO("Bind", this.opts.GateHandler.Bind)
	this.GetServer().RegisterGO("UnBind", this.opts.GateHandler.UnBind)
	this.GetServer().RegisterGO("Push", this.opts.GateHandler.Push)
	this.GetServer().RegisterGO("Set", this.opts.GateHandler.Set)
	this.GetServer().RegisterGO("Remove", this.opts.GateHandler.Remove)
	this.GetServer().RegisterGO("Send", this.opts.GateHandler.Send)
	this.GetServer().RegisterGO("SendBatch", this.opts.GateHandler.SendBatch)
	this.GetServer().RegisterGO("BroadCast", this.opts.GateHandler.BroadCast)
	this.GetServer().RegisterGO("IsConnect", this.opts.GateHandler.IsConnect)
	this.GetServer().RegisterGO("Close", this.opts.GateHandler.Close)
}

func (this *Gate) Run(closeSig chan bool) {
	var wsServer *network.WSServer
	if this.opts.WsAddr != "" {
		wsServer = new(network.WSServer)
		wsServer.Addr = this.opts.WsAddr
		wsServer.HTTPTimeout = 30*time.Second
		wsServer.Tls = this.opts.Tls
		wsServer.CertFile = this.opts.CertFile
		wsServer.KeyFile = this.opts.KeyFile
		wsServer.NewAgent = func(conn *network.WSConn) network.Agent {
			if this.createAgent == nil {
				this.createAgent = this.defaultCreateAgentd
			}
			agent := this.createAgent()
			agent.OnInit(this, conn)
			return agent
		}
	}

	var tcpServer *network.TCPServer
	if this.opts.TcpAddr != "" {
		tcpServer = new(network.TCPServer)
		tcpServer.Addr = this.opts.TcpAddr
		tcpServer.Tls = this.opts.Tls
		tcpServer.CertFile = this.opts.CertFile
		tcpServer.KeyFile = this.opts.KeyFile
		tcpServer.NewAgent = func(conn *network.TCPConn) network.Agent {
			if this.createAgent == nil {
				this.createAgent = this.defaultCreateAgentd
			}
			agent := this.createAgent()
			agent.OnInit(this, conn)
			return agent
		}
	}

	if wsServer != nil {
		wsServer.Start()
	}
	if tcpServer != nil {
		tcpServer.Start()
	}
	<-closeSig
	if this.opts.GateHandler != nil {
		this.opts.GateHandler.OnDestroy()
	}
	if wsServer != nil {
		wsServer.Close()
	}
	if tcpServer != nil {
		tcpServer.Close()
	}
}

func (this *Gate) OnDestroy() {
	this.BaseModule.OnDestroy() //这是必须的
}
