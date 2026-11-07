package controller

import (
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

type channelProxyTestRequest struct {
	Proxy string `json:"proxy"`
}

// TestChannelProxy 测试渠道代理地址的连通性：经代理访问探测地址，返回延迟与出口 IP / 国家。
// 支持多种代理写法（见 service.NormalizeProxyURL）。仅凭请求体中的地址探测，不涉及具体渠道；
// 绝不记录代理凭证，响应中的代理地址已打码。
func TestChannelProxy(c *gin.Context) {
	var req channelProxyTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "无效的请求参数"})
		return
	}
	if strings.TrimSpace(req.Proxy) == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "请填写代理地址"})
		return
	}

	result, err := service.ProbeProxy(c.Request.Context(), req.Proxy)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "代理地址格式无法识别：" + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    result,
	})
}
