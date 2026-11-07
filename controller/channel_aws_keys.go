package controller

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/model"
)

// AWS 渠道一个渠道只对应一组凭证（一个 AWS 账号），渠道内的多个 key 是这组凭证的不同区域。
// 不同账号混在一个渠道里，账号级故障（封号、AK 失效）只会被逐个 key 发现，重试会连带避开好账号，
// 用量统计、代理、并发上限也分不开。
//
// 密钥行格式与 relay/channel/aws newAwsClient 的解析一致：AK|SK|Region（3 段）或 APIKey|Region（2 段）。

type awsCredentialKeys struct {
	ID   string // AccessKey 或 APIKey，用于渠道命名
	Keys []string
}

// awsKeyCredential 取一行 AWS 密钥的凭证主体（去掉最后一段 Region）与 AccessKey/APIKey；格式不对返回 ok=false。
func awsKeyCredential(key string) (credential string, id string, ok bool) {
	parts := strings.Split(key, "|")
	if len(parts) != 2 && len(parts) != 3 {
		return "", "", false
	}
	for _, p := range parts {
		if strings.TrimSpace(p) == "" {
			return "", "", false
		}
	}
	return strings.Join(parts[:len(parts)-1], "|"), parts[0], true
}

// groupAwsKeysByCredential 把多行 AWS 密钥按凭证分组：组与组内顺序按首次出现，空行跳过，重复行去掉；
// 格式不对的行无法判断归属，直接报错（行号按原文本计，便于在密钥框里定位）。
func groupAwsKeysByCredential(text string) ([]awsCredentialKeys, error) {
	groups := make([]awsCredentialKeys, 0)
	groupIndex := make(map[string]int)
	seenLines := make(map[string]struct{})
	for i, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		credential, id, ok := awsKeyCredential(line)
		if !ok {
			return nil, fmt.Errorf("第 %d 行 AWS 密钥格式不正确，应为 AccessKey|SecretKey|Region 或 APIKey|Region", i+1)
		}
		if _, dup := seenLines[line]; dup {
			continue
		}
		seenLines[line] = struct{}{}
		idx, exists := groupIndex[credential]
		if !exists {
			idx = len(groups)
			groupIndex[credential] = idx
			groups = append(groups, awsCredentialKeys{ID: id})
		}
		groups[idx].Keys = append(groups[idx].Keys, line)
	}
	return groups, nil
}

// awsCredentialChannelName 多组凭证拆成多个渠道时的名称：原名-AccessKey 后四位。
func awsCredentialChannelName(name string, id string) string {
	suffix := id
	if len(id) > 4 {
		suffix = id[len(id)-4:]
	}
	if strings.TrimSpace(name) == "" {
		return suffix
	}
	return name + "-" + suffix
}

// buildAwsChannelsByCredential 聚合模式新建 AWS 渠道：每组凭证一个多 key 渠道，渠道内是该凭证的各区域 key。
// 只有一组凭证时名称不变；多组时各渠道名称追加 AccessKey 后四位以便区分。
func buildAwsChannelsByCredential(base model.Channel) ([]model.Channel, error) {
	groups, err := groupAwsKeysByCredential(base.Key)
	if err != nil {
		return nil, err
	}
	if len(groups) == 0 {
		return nil, errors.New("密钥不能为空")
	}
	channels := make([]model.Channel, 0, len(groups))
	for _, g := range groups {
		ch := base
		ch.Key = strings.Join(g.Keys, "\n")
		ch.ChannelInfo.IsMultiKey = true
		ch.ChannelInfo.MultiKeySize = len(g.Keys)
		if len(groups) > 1 {
			ch.Name = awsCredentialChannelName(base.Name, g.ID)
		}
		channels = append(channels, ch)
	}
	return channels, nil
}

// validateAwsChannelKey 校验提交的 AWS 渠道密钥：单 key 渠道只能一行（多行会被整串当一个 key 用而必然失败）；
// 多 key 渠道只能是一组凭证，多组凭证不合并，要新建渠道（聚合模式新建时每组凭证各建一个渠道）。
func validateAwsChannelKey(key string, isMultiKey bool) error {
	lines := 0
	for _, raw := range strings.Split(key, "\n") {
		if strings.TrimSpace(raw) != "" {
			lines++
		}
	}
	if !isMultiKey {
		if lines > 1 {
			return fmt.Errorf("AWS 单密钥渠道只能填写一行密钥（当前 %d 行）；多个区域的密钥请用「批量生成可用区域密钥」或开启「批量创建」", lines)
		}
		return nil
	}
	groups, err := groupAwsKeysByCredential(key)
	if err != nil {
		return err
	}
	if len(groups) > 1 {
		return fmt.Errorf("一个渠道只能使用一组 AWS 凭证（当前 %d 组），不会合并；多组凭证请新建渠道，每组凭证各建一个渠道", len(groups))
	}
	return nil
}
