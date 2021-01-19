// connKeeper
package server

import (
	"container/list"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"net"
	"sync"
	"time"

	"github.com/fatedier/frp/utils/xlog"

	"github.com/fatedier/frp/models/msg"
	gnet "github.com/fatedier/golib/net"
)

var (
	errConnPoolFull     = errors.New("Connection pool is full")
	errConnKeeperClosed = errors.New("Connection keeper is closed")
)

type keeper struct {
	connList    *list.List
	maxcount    int
	connCount   int32
	lock        sync.Locker
	waitTimeout time.Duration
	bClosed     bool
	logger      *xlog.Logger

	connReq     chan struct{}
	connResp    chan net.Conn
	connWait    chan struct{}
	controlChan chan (msg.Message)
}

type aliveConn struct {
	net.Conn
	c chan struct{}
}

func NewKeeper(MaxPoolCount int, ctlChan chan (msg.Message), timeout int64, logger *xlog.Logger) *keeper {
	k := &keeper{
		connList:    list.New(),
		connCount:   0,
		maxcount:    MaxPoolCount,
		connReq:     make(chan struct{}, 999),
		connResp:    make(chan net.Conn, 999),
		connWait:    make(chan struct{}),
		controlChan: ctlChan,
		lock:        &sync.Mutex{},
		logger:      logger,
		waitTimeout: time.Second * time.Duration(timeout),
	}
	go k.connFactory()
	return k
}

func (k *keeper) activeWait() {
	select {
	case k.connWait <- struct{}{}:
	default:
	}
}

func (k *keeper) requireWorkConn() {
	defer func() {
		if e := recover(); e != nil {
			k.logger.Info("%v", e)
		}
	}()
	if !k.bClosed {
		k.controlChan <- &msg.ReqWorkConn{}
	}
}

func (k *keeper) AddConn(c net.Conn) error {
	k.lock.Lock()
	defer k.lock.Unlock()
	if k.bClosed {
		return errConnKeeperClosed
	}
	if k.connList.Len() >= k.maxcount+10 { // 允许10个额外的连接
		return errConnPoolFull
	}

	// 对于tcp连接（目前都是），设置keepalive并开始守护
	switch c.(type) {
	case *fushionConn:
		{
			c = &aliveConn{
				Conn: c,
				c:    make(chan struct{}),
			}
			go k.keepalive(c.(*aliveConn))
		}
	case *gnet.SharedConn:
		{
			c = &aliveConn{
				Conn: c,
				c:    make(chan struct{}),
			}
			go k.keepalive(c.(*aliveConn))
		}
	}
	k.connList.PushBack(c)
	k.activeWait() // 唤醒等待新连接的factory routine
	if k.maxcount > 0 {
		log.Println(k.connList.Len(), "in pool, max", k.maxcount)
		log.Println(k.connList.Len(), "in pool, max", k.maxcount)
	}
	return nil
}

func (k *keeper) keepalive(c *aliveConn) {
	defer func() {
		close(c.c)
	}()
	_, e := io.Copy(ioutil.Discard, c.Conn)
	if k.bClosed { // keeper已停止，忽略一切错误
		return
	}

	if op, ok := e.(*net.OpError); !ok || !op.Timeout() {
		// 该连接异常断开，从pool中删除连接并申请一个新连接
		k.lock.Lock()
		defer k.lock.Unlock()
		log.Println("[keepalive] error", e)
		c.Close()
		for it := k.connList.Front(); it.Next() != nil; it = it.Next() {
			if it.Value == c {
				k.connList.Remove(it) // 不用去管connWait，下次尝试会自然失败
				break
			}
		}
		if k.connList.Len() < k.maxcount {
			k.requireWorkConn() // 发起新连接申请
		}
		return
	}
	c.SetDeadline(time.Time{}) // 连接被使用，取消deadline
}

func (k *keeper) popConn() net.Conn {
	k.lock.Lock()
	defer k.lock.Unlock()
	k.logger.Info("popping net.Conn")
	if k.connList.Len() > 0 { // pool 中还在连接
		c := k.connList.Remove(k.connList.Front()).(net.Conn)
		if conn, ok := c.(*aliveConn); ok { // 是正在keepalive的conn，取消keepalive，并等在取消完成
			conn.SetReadDeadline(time.Now())
			<-conn.c
			c = conn.Conn
		}
		if k.connList.Len() > 0 {
			k.activeWait() // 如果还有剩余的连接，持续唤醒
		}
		if k.connList.Len() < k.maxcount && !k.bClosed {
			k.requireWorkConn()
		}
		k.logger.Info("here you go, %d in stock", k.connCount)
		return c
	} else {
		k.logger.Info("we're exhausted")
		return nil
	}
}

func (k *keeper) connFactory() {
	for {
		if _, ok := <-k.connReq; !ok { // 等待获取连接请求
			return
		}
		go func() {
			c := k.popConn() // 尝试从pool中直接获得一个conn
			if c == nil && k.maxcount == 0 {
				k.requireWorkConn() // 没有pool，必须直接获得连接
			}
			if c == nil {
				t := time.NewTimer(k.waitTimeout)
				for {
					select {
					case <-t.C:
						{
							k.logger.Warn("timeout getting a working connection")
							k.connResp <- nil // 等待超时，放弃获取连接，返回一个nil
							return
						}
					case <-k.connWait: // 获得了一个机会
						{
							if c = k.popConn(); c != nil { // 成功抢到了连接，停止计时并返回
								t.Stop()
								k.connResp <- c
								return
							}
						}
					}
				}
			} else {
				k.connResp <- c
			}
		}()
	}
}

func (k *keeper) GetConn() (c net.Conn, success bool) {
	defer func() {
		if err := recover(); err != nil {
			c = nil
			success = false
		}
	}()
	k.connReq <- struct{}{} // 请求获得一个连接
	c = <-k.connResp        // 等待连接, 在超时前必定会获得一个结果，失败则是nil
	return c, c != nil
}

func (k *keeper) Stop() {
	k.lock.Lock()
	defer k.logger.Info("ConnKeeper stopped")
	k.logger.Info("ConnKeeper stopping")
	k.bClosed = true
	close(k.connReq) // stop connFactory
	k.lock.Unlock()
	for c := k.popConn(); c != nil; c = k.popConn() { // 关闭所有连接，同时会自动禁用所有keepalive
		k.logger.Info("closing")
		c.Close()
	}
	k.logger.Info("ok")
}
