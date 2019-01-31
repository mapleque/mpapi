package mpapi

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/mapleque/kelp/logger"
	"github.com/mapleque/kelp/mysql"
)

const (
	MP_NOTIFY_STATUS_NEW     = 0
	MP_NOTIFY_STATUS_DOING   = 1
	MP_NOTIFY_STATUS_SUCCESS = 2
	MP_NOTIFY_STATUS_FAILD   = 3
)

type NotifyServer struct {
	conn mysql.Connector
	log  logger.Loggerer
}

func NewNotifyServer(conn mysql.Connector) *NotifyServer {
	server := &NotifyServer{
		conn: conn,
		log:  logger.Get("http"),
	}
	return server
}

func (this *NotifyServer) StartNotify(maxThread int) {
	if maxThread < 1 {
		return
	}
	nextSig := make(chan bool, 1)
	nextSig <- true
	for {
		select {
		case <-nextSig:
			if this.doNotifyBatch(maxThread) > 0 {
				nextSig <- true
			} else {
				time.Sleep(10 * time.Second)
				nextSig <- true
			}
		}
	}
}

type NotifyTask struct {
	// query from mp_notify
	Id              int64  `json:"-" column:"id"`
	UserId          int64  `json:"-" column:"user_id"`
	AppId           string `json:"-" column:"appid"`
	TemplateId      string `json:"template_id"`
	Page            string `json:"page"`
	FormId          string `json:"form_id"`
	Datastr         string `json:"-" column:"data"`
	EmphasisKeyword string `json:"emphasis_keyword"`
	Status          int    `json:"-" column:"status"`
	Additional      string `json:"-" column:"additional"`
	ActiveAt        string `json:"-" column:"active_at"`
	CreateAt        string `json:"-" column:"create_at"`

	// join from user_open
	OpenId string `json:"touser" column:"openid"`

	// build from Datastr
	Data map[string]interface{} `json:"data" column:"-"`
}

func (this *NotifyServer) doNotifyBatch(maxTaskNum int) int {
	taskList := []*NotifyTask{}
	if err := this.conn.Query(
		&taskList,
		"SELECT mp_notify.id AS id,  mp_notify.user_id AS user_id, mp_notify.appid AS appid, "+
			"template_id, page, form_id, data, emphasis_keyword, status, additional, active_at, "+
			"mp_notify.create_at AS create_at, openid "+
			"FROM mp_notify INNER JOIN user "+
			"ON mp_notify.user_id = user.id "+
			"WHERE mp_notify.status = ? AND mp_notify.active_at <= NOW() "+
			"ORDER BY mp_notify.id LIMIT ?",
		MP_NOTIFY_STATUS_NEW,
		maxTaskNum,
	); err != nil {
		return 0
	}
	this.log.Info("wx notify start", len(taskList))
	var wg sync.WaitGroup
	for _, task := range taskList {
		wg.Add(1)
		go func(task *NotifyTask) {
			defer wg.Done()
			if eff, err := this.conn.Execute(
				"UPDATE mp_notify SET status = ? WHERE id = ? AND status = ? LIMIT 1",
				MP_NOTIFY_STATUS_DOING,
				task.Id,
				MP_NOTIFY_STATUS_NEW,
			); err != nil || eff != 1 {
				return
			}
			if wxapp, err := NewWXApp(task.AppId, this.conn); err != nil {
				this.log.Error("can not create wx app", err)
				this.conn.Execute(
					"UPDATE mp_notify SET status = ?, additional = ? WHERE id = ? AND status = ? LIMIT 1",
					MP_NOTIFY_STATUS_FAILD,
					"invalid appid",
					task.Id,
					MP_NOTIFY_STATUS_DOING,
				)
			} else {
				task.Data = map[string]interface{}{}
				if err := json.Unmarshal([]byte(task.Datastr), &task.Data); err != nil {
					this.log.Error("message data encode error", task.Datastr)
					this.conn.Execute(
						"UPDATE mp_notify SET status = ?, additional = ? WHERE id = ? AND status = ? LIMIT 1",
						MP_NOTIFY_STATUS_FAILD,
						"message data decode error",
						task.Id,
						MP_NOTIFY_STATUS_DOING,
					)
					return
				}
				body, _ := json.Marshal(task)
				if err := wxapp.SendTemplateMessage(body); err != nil {
					this.log.Error("send message error", err)
					this.conn.Execute(
						"UPDATE mp_notify SET status = ?, additional = ? WHERE id = ? AND status = ? LIMIT 1",
						MP_NOTIFY_STATUS_FAILD,
						err.Error(),
						task.Id,
						MP_NOTIFY_STATUS_DOING,
					)
				} else {
					this.conn.Execute(
						"UPDATE mp_notify SET status = ? WHERE id = ? AND status = ? LIMIT 1",
						MP_NOTIFY_STATUS_SUCCESS,
						task.Id,
						MP_NOTIFY_STATUS_DOING,
					)
				}
			}
		}(task)
	}
	wg.Wait()
	this.log.Info("wx notify end", len(taskList))
	return len(taskList)
}
