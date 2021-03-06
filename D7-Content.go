package main

import (
	"github.com/Ballwang/tugo/tool"
	"strconv"
	"fmt"
	"net/http"
	"github.com/Ballwang/tugo/config"
	"github.com/chasex/redis-go-cluster"
	"encoding/json"
	"time"
	"runtime"
)

var contentChan chan int = make(chan int)

type DataSource struct {
	Nodeid string
	Url    string
	Msg    string
}

//开始获取网站内容
func StartGetContent(w http.ResponseWriter, req *http.Request) {
	runtime.GOMAXPROCS(config.MaxCpu)
	for {
		c, err := tool.NewRedisCluster()
		if err != nil {
			fmt.Println(err)
		}

		url := []string{}
		for i := 0; i < config.MaxContentProcess; i++ {
			r, err := c.Do("SPOP", config.ContentUrlSet)
			if err != nil {

			}
			s, _ := redis.String(r, nil)
			url = append(url, s)
		}
		processCount := len(url)
		if processCount >= 0 {
			for _, v := range url {
				go GetContent(c, v)
			}
			for i := 0; i < processCount; i++ {
				<-contentChan
			}
		}
		c.Close()
		time.Sleep(1 * time.Second)
		tool.SetServerState("D7-Content", "22")
	}
}

//获取服务状态
func GetContentState(w http.ResponseWriter, req *http.Request) {
	fmt.Fprint(w, tool.GetServerState("D7-Content"))
}

//获取链接内容
func GetContent(c *redis.Cluster, url string) {
	state, content := tool.HttpRequestContent(url)
	if state == 200 {
		parentUrl, ok := tool.RedisClusterGetHashValueAndDelWithReids(c,config.ContentParentHash, url)
		if ok {
			nodeid, ok := tool.RedisClusterGetHashValueWithRedis(c,config.MonitorHash, config.Nodeid+parentUrl)
			if ok {
				dataSource := &DataSource{}
				dataSource.Nodeid = nodeid
				dataSource.Url = url
				dataSource.Msg = content
				b, err := json.Marshal(dataSource)
				if err != nil {

				}
				c.Do("RPUSH", config.DataSource, b)
			}
		}
	}
	contentChan <- 1
}

func main() {

	ip := tool.GetIP()
	var serverID = "D7-Content:"+ip
	config:=config.NewConfig()
	serverPort,_:=strconv.Atoi(config.GetConfig("D7-Content","port"))

	http.HandleFunc("/D7-Content", StartGetContent)
	http.HandleFunc("/State", GetContentState)
	register := &tool.ConsulRegister{Id: serverID, Name: "D7-Content", Port: serverPort, Tags: []string{"D7-能够或许详细的更新内容！"}}
	register.RegisterConsulService()
	err := http.ListenAndServe(ip+":"+strconv.Itoa(serverPort), nil)

	if err != nil {
		fmt.Println("Listen And Serve error: ", err.Error())
	}

}
