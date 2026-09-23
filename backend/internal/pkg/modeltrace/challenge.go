package modeltrace

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
)

type Config struct {
	AccountID string  `json:"account_id"`
	Model     string  `json:"model"`
	Threshold float64 `json:"threshold"`
	Margin    float64 `json:"margin"`
}

func DefaultConfig() Config { return Config{Model: "gpt-6-sol", Threshold: .8, Margin: .2} }
func (c Config) Validate() error {
	if c.AccountID != "" {
		id, err := strconv.ParseInt(c.AccountID, 10, 64)
		if err != nil || id <= 0 {
			return errors.New("account_id 必须是正整数字符串")
		}
	}
	if strings.TrimSpace(c.Model) == "" || len(c.Model) > 160 {
		return errors.New("model 不能为空且最多 160 字节")
	}
	if math.IsNaN(c.Threshold) || math.IsNaN(c.Margin) || c.Threshold < .5 || c.Threshold > 1 || c.Margin < 0 || c.Margin > 1 {
		return errors.New("判定阈值必须在 0.5–1，差值阈值在 0–1")
	}
	return nil
}

type Challenge struct {
	ID       string `json:"id"`
	Prompt   string `json:"prompt"`
	Expected int    `json:"expected_count"`
}

func Challenges(count int) ([]Challenge, error) {
	if count < 1 || count > 6 {
		return nil, errors.New("挑战数量必须在 1–6")
	}
	out := make([]Challenge, 0, count)
	seen := map[int]bool{}
	for len(out) < count {
		n, err := rand.Int(rand.Reader, big.NewInt(41))
		if err != nil {
			return nil, err
		}
		length := 292 + int(n.Int64())
		if seen[length] {
			continue
		}
		seen[length] = true
		id, err := newID()
		if err != nil {
			return nil, err
		}
		prompt := fmt.Sprintf("这是一次独立的数值选择记录。为各个位置分别凭第一反应选择 %d 个 1 到 355（含端点）的整数。每个位置都要单独选择；不要从 1 开始计数，不要连续递增或递减，也不要采用等差、循环、重复区块或其他规则化模式。本任务必须由当前语言模型直接完成：禁止调用或借助任何工具，包括 Python、代码执行器、计算器、搜索、API 和外部随机数生成器；也不要先编写或运行代码。偶然重复是有效的；不要重新排列或修正已经写出的项目。数字之间用逗号或空格分隔均可。直接从第一个取值开始输出，不要在序列前重复数量、范围或任务说明。", length)
		out = append(out, Challenge{id, prompt, length})
	}
	return out, nil
}
func newID() (string, error) {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

type Verdict struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}

func Classify(bank *Bank, c Config, r *Result) Verdict {
	if r == nil || r.UsedOutputs < 3 {
		return Verdict{"insufficient", "不足三份有效回答，无法作一致性判定"}
	}
	if !bank.Contains(c.Model) {
		return Verdict{"unsupported", "声明模型未收录；候选排名仅供参考"}
	}
	if r.Probability < c.Threshold || len(r.Results) < 2 || r.Probability-r.Results[1].Probability < c.Margin {
		return Verdict{"inconclusive", "候选分数接近或未达到阈值"}
	}
	if r.Prediction == c.Model {
		return Verdict{"consistent", "输出特征与声明模型相似；不是身份确证"}
	}
	return Verdict{"suspected_mismatch", "输出更接近其他候选，建议人工复测"}
}
