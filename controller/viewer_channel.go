package controller

import (
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// viewerChannelInfo 观察员可见的多 Key 摘要：只暴露是否多 Key 与数量，不暴露各 Key 状态/轮询下标。
type viewerChannelInfo struct {
	IsMultiKey   bool `json:"is_multi_key"`
	MultiKeySize int  `json:"multi_key_size"`
}

// viewerChannel 观察员渠道白名单 DTO。key / base_url / 成本价 / 应收款 / 各类覆写 / 备注 / 权重优先级 /
// 供应商与创建者身份一律不进入此结构体，因此不可能被序列化出去。新增字段前先对照设计文档 §1.1。
type viewerChannel struct {
	Id                 int               `json:"id"`
	Name               string            `json:"name"`
	Type               int               `json:"type"`
	Status             int               `json:"status"`
	Models             string            `json:"models"`
	Group              string            `json:"group"`
	ResponseTime       int               `json:"response_time"`
	TestTime           int64             `json:"test_time"`
	CreatedTime        int64             `json:"created_time"`
	UsedQuota          int64             `json:"used_quota"`
	Balance            float64           `json:"balance"`
	BalanceUpdatedTime int64             `json:"balance_updated_time"`
	ChannelInfo        viewerChannelInfo `json:"channel_info"`
	// Tag 供前端标签聚合模式分组渲染；无标签时省略。
	Tag string `json:"tag,omitempty"`
}

func viewerChannelView(ch *model.Channel) *viewerChannel {
	if ch == nil {
		return nil
	}
	v := &viewerChannel{
		Id:                 ch.Id,
		Name:               ch.Name,
		Type:               ch.Type,
		Status:             ch.Status,
		Models:             ch.Models,
		Group:              ch.Group,
		ResponseTime:       ch.ResponseTime,
		TestTime:           ch.TestTime,
		CreatedTime:        ch.CreatedTime,
		UsedQuota:          ch.UsedQuota,
		Balance:            ch.Balance,
		BalanceUpdatedTime: ch.BalanceUpdatedTime,
		ChannelInfo: viewerChannelInfo{
			IsMultiKey:   ch.ChannelInfo.IsMultiKey,
			MultiKeySize: ch.ChannelInfo.MultiKeySize,
		},
	}
	if ch.Tag != nil {
		v.Tag = *ch.Tag
	}
	return v
}

func viewerChannelViews(channels []*model.Channel) []*viewerChannel {
	out := make([]*viewerChannel, 0, len(channels))
	for _, ch := range channels {
		if ch == nil {
			continue
		}
		out = append(out, viewerChannelView(ch))
	}
	return out
}

// ViewerListChannels 观察员渠道列表：复用管理员列表核心，输出白名单 DTO。
func ViewerListChannels(c *gin.Context) {
	listChannelsCore(c, channelListOptions{viewer: true})
}

// ViewerSearchChannels 观察员渠道搜索：复用管理员搜索核心，输出白名单 DTO。
func ViewerSearchChannels(c *gin.Context) {
	searchChannelsCore(c, channelListOptions{viewer: true})
}
