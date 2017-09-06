package main

import (
	"github.com/Ballwang/tugo/tool"
	"github.com/Ballwang/tugo/config"
	"time"
	"github.com/axgle/mahonia"
	"github.com/garyburd/redigo/redis"
	"strings"
	"fmt"
)

var completeCat chan int = make(chan int)
var textLen = 24 //8个汉字
//列表页详细链接提取
func main() {

	for {
		c, _ := tool.NewRedis()
		IsEnd := 0
		for {
			SiteList := []string{}
			//for i := 0; i < config.MaxProcess; i++ {
			for i := 0; i < 100; i++ {
				reply, err := c.Do("LPOP", config.UpdateList)
				tool.Error(err)
				if reply != nil {
					SiteList = append(SiteList, string(reply.([]byte)))
				} else {
					IsEnd = 1
					//退出当前循环
					break
				}
			}
			process := len(SiteList)
			if process > 0 {
				for _, v := range SiteList {
					//进入并发程序
					go GetUrlFromCategory(v)
				}
				for i := 0; i < process; i++ {
					<-completeCat
				}
			}

			if IsEnd == 1 {
				break
			}
			break
		}
		//test
		break
		c.Close()
		time.Sleep(1 * time.Second)
	}
}

//从栏目页提取链接
func GetUrlFromCategory(url string) {
	c, _ := tool.NewRedis()
	html, _ := c.Do("Get", config.PrefixCategory+url)
	if html != nil {
		//获取链接提取规则
		urlString, _ := c.Do("HGET", config.MonitorShowHash, url)
		content, _ := c.Do("GET", config.PrefixCategory+url)
		//转化为小写

		var contentString string
		var urlParent string

		if urlString != nil {
			urlParent, _ = redis.String(urlString, nil)
		} else {
			urlParent = url
		}

		//urlStart, _ := redis.String(c.Do("HGET", config.MonitorHash, config.UrlStart+url))
		//urlEnd, _ := redis.String(c.Do("HGET", config.MonitorHash, config.UrlEnd+url))
		//获取链接配置信息
		Sourcecharset, _ := redis.String(c.Do("HGET", config.MonitorHash, config.Sourcecharset+urlParent))

		if !strings.Contains(Sourcecharset, "utf") {
			dec := mahonia.NewDecoder("gbk")
			contentString = dec.ConvertString(string(content.([]byte)))
		} else {
			contentString, _ = redis.String(content, nil)
		}
		//开始链接提取
		contentString=strings.ToLower(contentString)

		resultMap := map[string]string{}

		result := tool.ListHref(contentString)

		//fmt.Print(contentString)

		//分析网站链接是否为文章链接
		if len(result) > 0 {
			for _, v := range result {
				if v != "" {
					aText := tool.GetALabelText(v)
					if len(aText) > textLen {
						resultMap[v] = aText
					}
				} else {

				}
			}
		}

		//分析链接特征
		count := map[int]int{}
		countMap := map[string]int{}
		countSubfix := map[string]int{}
		if len(resultMap) > 0 {
			for k, _ := range resultMap {
				url := tool.GetALabelUrl(k)
				subfix := tool.GetHostUrlSuffix(url)
				urlNoHost:=tool.RemovHostNameByUrl(url)
				urlArray := strings.Split(urlNoHost, "/")
				if len(urlArray) > 0 {
					count[len(urlArray)]++
					countMap[url] = len(urlArray)
					//fmt.Print(url)
					//fmt.Print("-----------")
					//fmt.Print( len(urlArray))
					//fmt.Print("\n")
				}
				if subfix != "" {
					subArray := strings.Split(url, subfix)
					if len(subArray) > 0 {
						countSubfix[subfix]++
					}
				} else {
					countSubfix["nilSubfix"]++
				}
			}

			//筛选权重最高的链接形式
			maxLen := 0
			secondLen :=0
			maxKey := 0
			secondKey :=0
			if len(count) > 0 {
				for k, v := range count {
					if v > maxLen {
						secondLen=maxLen
						secondKey=maxKey
						maxLen = v
						maxKey = k
					}
					if v>secondLen&&v<maxLen{
						secondLen=v
						secondKey=k
					}
				}
			}
			//筛选使用最多的后缀
			maxLenSubfix := 0
			maxkeySubfix := ""
			if len(countSubfix) > 0 {
				for key, v := range countSubfix {
					if v > maxLenSubfix {
						maxLenSubfix = v
						maxkeySubfix = key
					}

				}
			}
			//判断页面查集
			//fmt.Print(maxLen-secondLen)
			//fmt.Print("\n")
			//
			//fmt.Print(maxKey)
			//fmt.Print("\n")
			//
			//fmt.Print(secondKey)
			//fmt.Print("\n")
			//




			//筛选结束
			fmt.Print(len(resultMap))


			c, _ := tool.NewRedis()
			result := []string{}
			if len(countMap) > 0 && maxLen >= config.MinNumOfUrl {
				for k, v := range countMap {
					//兼容网页侧边栏相似链接现象
					isDo:=false
					if (maxLen-secondLen)<=config.MaxMinusValue{
						if v == maxKey||v==secondKey{
							isDo=true
						}
					}else {
						if  v == maxKey{
							//fmt.Print(maxKey)
							//fmt.Print("@@@@@@@@@@@@@\n")
							//fmt.Print(v)
							//fmt.Print("____________________\n")
							isDo=true
						}
					}
					if isDo {
						if maxkeySubfix != "" {
							if strings.Contains(k, maxkeySubfix) {
								result = append(result, k)
								AbsoluteUrl := tool.GetAbsoluteUrl(url, k)
								historyKey := tool.GetHostUri(url)
								md5String := tool.Md5String(AbsoluteUrl)
								key := config.HistoryPrefix + historyKey
								r, _ := c.Do("SISMEMBER", key, md5String)
								var j int64=1
								if r != j {
									c.Do("SADD", key, md5String)
									fmt.Print(AbsoluteUrl)
									fmt.Print("\n")
									c.Do("HSET", config.ContentParentHash, AbsoluteUrl, urlParent)
									c.Do("SADD", config.ContentUrlSet, AbsoluteUrl)
								}
								if historyKey==""{
									c.Do("SADD",config.NullKey,url)
								}
							}
						}else {
							c.Do("SADD",config.NullUrlInCategory,urlParent)
						}
					}
				}
			}
			fmt.Print(url)
		}
	}
	completeCat <- 1
}
