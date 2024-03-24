// rislive包提供对rislive的数据获取
package rislive

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

const HEARTBEATETIME = time.Duration(40) * time.Second //心跳发送时间间隔

// RISLive消息交互基础格式
type RisLiveMessage[T RisLiveServerMessageData | RisLiveClientMessageData] struct {
	Type string `json:"type,omitempty"` //RISLive 消息类型
	Data *T     `json:"data,omitempty"` //RISLive 消息数据，根据类型不同而不同
}

// RISLive客户端消息数据格式，并不是每一个域都会用到，一般用于通知服务器仅接收哪些路由信息
type RisLiveClientMessageData struct {
	Host    string `json:"host,omitempty"`    //只接收指定路由信息收集器的消息
	Type    string `json:"type,omitempty"`    //只接受指定类型的BGP消息，可选的值有"UPDATE" "OPEN" "NOTIFICATION" "KEEPALIVE" "RIS_PEER_STATE"
	Require string `json:"require,omitempty"` //只接受包含给定key的消息，可选的值有"announcements" "withdrawals"
}

// 创建一个客户端发向服务端的订阅请求
func MakeRisLiveSubscribe(filter *RisLiveClientMessageData) *RisLiveMessage[RisLiveClientMessageData] {
	return &RisLiveMessage[RisLiveClientMessageData]{
		Type: "ris_subscribe",
		Data: filter,
	}
}

// 创建一个客户端发向服务端的取消订阅请求
func MakeRisUnsubscribe(filter *RisLiveClientMessageData) *RisLiveMessage[RisLiveClientMessageData] {
	return &RisLiveMessage[RisLiveClientMessageData]{
		Type: "ris_unsubscribe",
		Data: filter,
	}
}

// 向服务端获取Hosts列表
func MakeRisRequestRrcList() *RisLiveMessage[RisLiveClientMessageData] {
	return &RisLiveMessage[RisLiveClientMessageData]{
		Type: "request_rrc_list",
	}
}

// 向服务端发送Ping请求，服务端应当返回pong。
// 用于检测连接
func MakeRisPing() *RisLiveMessage[RisLiveClientMessageData] {
	return &RisLiveMessage[RisLiveClientMessageData]{
		Type: "ping",
	}
}

// RISLive服务器端消息数据格式，并不是每一个域都会用到
// 处于解析方便，这里只保留Path和Announcements
type RisLiveServerMessageData struct {
	Path          []RisLivePath          `json:"path,omitempty"`          //来自AS_PATH，从first上游开始，可能以AS_SET结束；值可能是[1,2,3,[64496,64497]]
	Announcements []*RisLiveAnnouncement `json:"announcements,omitempty"` //BGP通告
	//	Timestamp     float64                `json:"timestamp"`               //RIS 路由信息收集器收到消息的时间
	//	Peer          string                 `json:"peer"`                    //发送此消息的BGP对等点的IP地址（IPV4/IPV6）
	//	PeerASN       string                 `json:"peer_asn"`                //发送此消息的BGP对等点的自治域号
	//	ID            string                 `json:"id"`                      //消息的唯一标识符，保证特定的RIS会话按顺序排序
	//	Raw           string                 `json:"raw,omitempty"`           //原始BGP消息的网络字节序的十六进制编码，当且仅当socketOptions.includeRaw时才存在
	//	Host          string                 `json:"host,omitempty"`          //接收此消息的RIS路由信息收集器的主机名
	//	Type          string                 `json:"type"`                    //可能存在的值"UPDATE" "KEEPALIVE" "OPEN" "NOTIFICATION" "RIS_PEER_STATE"
	//	Community     [][]int32              `json:"community,omitempty"`     //社团路径属性将ASN与社团值配对，值可能是[[64496,1111],[64497,2222]]
	//	Origin        string                 `json:"origin,omitempty"`        //起源路径属性，如果存在的话，比如igp，incomplete，egp
	//	Med           int                    `json:"med,omitempty"`           //MULTI_EXIT_DISC路径属性，如果存在的话
	//	Aggregator    string                 `json:"aggregator,omitempty"`    //聚合器属性，如果存在的话
}

type RisLivePath int

// 如果路径中出现ASset，则用-1表示，因为本程序不处理ASset的情况
func (r *RisLivePath) UnmarshalJSON(buf []byte) error {
	if asn, err := strconv.Atoi(string(buf)); err == nil {
		*r = RisLivePath(asn)
	} else {
		//出现ASset
		*r = -1
	}
	return nil
}

// 从RISLive服务端消息数据中获取起源AS
// 如果无法获取单一起源AS则返回-1
func (r *RisLiveServerMessageData) GetOriginAS() int {
	pathLen := len(r.Path)
	if pathLen <= 0 {
		return -1
	}
	return int(r.Path[pathLen-1])
}

// 从RISLive服务端消息数据中获取前缀
func (r *RisLiveServerMessageData) GetPrefixes(onlyIPv4 bool) []string {
	res := []string{}
	if onlyIPv4 {
		for _, announcement := range r.Announcements {
			for _, prefix := range announcement.Prefixes {
				if isIpv4(prefix) {
					res = append(res, prefix)
				}
			}
		}
	} else {
		for _, announcement := range r.Announcements {
			res = append(res, announcement.Prefixes...)
		}
	}
	return res
}

// 区分IPV4与IPV6前缀
func isIpv4(prefix string) bool {
	return strings.Contains(prefix, ".")
}

// Ris传递的BGP通告
type RisLiveAnnouncement struct {
	Prefixes []string `json:"prefixes"` //任何ipv4，ipv6无类域间路由前缀
	// NextHop  string   `json:"next_hop"` //下一条
	// Withdrawals []string `json:"withdrawals,omitempty"` //不知道怎么使用
}

// 创建RisLive订阅Url
func MakeRisLiveSubscribeUrl(clientId string) string {
	values := url.Values{}
	values.Add("client", clientId)
	url := url.URL{
		Scheme:   "wss",
		Host:     "ris-live.ripe.net:443",
		Path:     "/v1/ws/",
		RawQuery: values.Encode(),
	}
	return url.String()
}

// RisLive信息处理Handle
type RisLiveHandle struct {
	mu           sync.Mutex
	conn         *websocket.Conn //websocket连接
	messageCount int64           //接收到的message数量

	subscribeUrl string
	dead         int32                                         // set by Kill()
	Channel      chan RisLiveMessage[RisLiveServerMessageData] //接收RisLive服务端消息的管道
	// 计时器
	taskTimer      *time.Timer //任务计时器
	heartbeatTimer *time.Timer //心跳计时器
}

// 创建RisLive处理Handle
func MakeRisLiveHandle(subscribeUrl string, filter *RisLiveClientMessageData, duration int) (*RisLiveHandle, error) {
	// 创建websocket连接
	conn, _, err := websocket.DefaultDialer.Dial(subscribeUrl, nil)
	if err != nil {
		return nil, err
	}
	slog.Info("create conn", "localAddr", conn.LocalAddr(), "remoteAddr", conn.RemoteAddr())
	risLiveHandle := &RisLiveHandle{
		conn:         conn,
		subscribeUrl: subscribeUrl,
		messageCount: 0,
		Channel:      make(chan RisLiveMessage[RisLiveServerMessageData], 1024),
		//计时器初始化
		taskTimer:      time.NewTimer(time.Duration(duration) * time.Second),
		heartbeatTimer: time.NewTimer(HEARTBEATETIME),
	}
	go risLiveHandle.receive(filter)
	go risLiveHandle.ticker()
	return risLiveHandle, nil
}

// 计时器，保证
func (handle *RisLiveHandle) ticker() {
	for !handle.killed() {
		select {
		case <-handle.taskTimer.C: //任务超时
			handle.Kill()
		case <-handle.heartbeatTimer.C: //心跳超时,发送Ping请求，保持连接
			if handle.killed() {
				return
			}
			handle.mu.Lock()
			slog.Info("send Ping to Ris")
			if err := handle.conn.WriteJSON(MakeRisPing()); err != nil {
				slog.Error(err.Error())
			}
			handle.heartbeatTimer.Reset(HEARTBEATETIME)
			handle.mu.Unlock()
		}
	}
}

// 从websocket端接收的消息
func (handle *RisLiveHandle) receive(filter *RisLiveClientMessageData) {
	if err := handle.conn.WriteJSON(MakeRisLiveSubscribe(filter)); err != nil {
		slog.Error(err.Error())
		return
	}
	for !handle.killed() {
		var msg RisLiveMessage[RisLiveServerMessageData]
		handle.conn.SetReadDeadline(time.Now().Add(time.Duration(10)*time.Second))
		err := handle.conn.ReadJSON(&msg)
		if err != nil {
			slog.Info(err.Error())
			handle.mu.Lock()
			conn, _, err := websocket.DefaultDialer.Dial(handle.subscribeUrl, nil)
			if err != nil {
				slog.Error(err.Error())
				handle.mu.Unlock()
				return
			}
			handle.conn = conn
			if err := handle.conn.WriteJSON(MakeRisLiveSubscribe(filter)); err != nil {
				slog.Error(err.Error())
				handle.mu.Unlock()
				return
			}
			handle.mu.Unlock()
			slog.Info("recreate conn", "localAddr", handle.conn.LocalAddr(), "remoteAddr", handle.conn.RemoteAddr())
		} else {
			handle.messageCount++
			handle.Channel <- msg
		}
	}
}

// 终止Handle
func (handle *RisLiveHandle) Kill() {
	atomic.StoreInt32(&handle.dead, 1)
}

// 检查Handle是否已经停止
func (handle *RisLiveHandle) killed() bool {
	z := atomic.LoadInt32(&handle.dead)
	return z == 1
}

// 处理接收的消息
func (handle *RisLiveHandle) Process(onlyIPv4 bool) {
	f, _ := os.Create("result")
	f.WriteString("asn ip \n")
	for !handle.killed() {
		var msg = <-handle.Channel
		if msg.Type == "pong" { //pong消息忽略
			slog.Info("receive Pong from Ris")
			continue
		}
		originAS := msg.Data.GetOriginAS()
		prefixes := msg.Data.GetPrefixes(false)
		if originAS != -1 {
			for _, prefix := range prefixes {
				f.WriteString(fmt.Sprintf("%d %s \n", originAS, prefix))
			}
		}
	}
}
