package middleware

import (
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/model"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

const (
	// SecureVerificationSessionKey 安全验证的 session key（与 controller 保持一致）
	SecureVerificationSessionKey       = "secure_verified_at"
	secureVerificationMethodSessionKey = "secure_verified_method"
	// SecureVerificationTimeout 验证有效期（秒）
	SecureVerificationTimeout = 300 // 5分钟
)

// SecureVerificationRequired 安全验证中间件
// 检查用户是否在有效时间内通过了安全验证
// 如果未验证或验证已过期，返回 401 错误
func SecureVerificationRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 检查用户是否已登录
		userId := c.GetInt("id")
		if userId == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "未登录",
			})
			c.Abort()
			return
		}

		// 检查 session 中的验证时间戳
		session := sessions.Default(c)
		verifiedAtRaw := session.Get(SecureVerificationSessionKey)

		if verifiedAtRaw == nil {
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": "需要安全验证",
				"code":    "VERIFICATION_REQUIRED",
			})
			c.Abort()
			return
		}

		verifiedAt, ok := verifiedAtRaw.(int64)
		if !ok {
			// session 数据格式错误
			clearSecureVerificationSession(session)
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": "验证状态异常，请重新验证",
				"code":    "VERIFICATION_INVALID",
			})
			c.Abort()
			return
		}

		// 检查验证是否过期
		elapsed := time.Now().Unix() - verifiedAt
		if elapsed >= SecureVerificationTimeout {
			// 验证已过期，清除 session
			clearSecureVerificationSession(session)
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": "验证已过期，请重新验证",
				"code":    "VERIFICATION_EXPIRED",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireTwoFAEnabled 轻量校验中间件：仅要求当前用户「已启用 2FA」或「已注册 Passkey」，
// 不做 session 级输码挑战（区别于 SecureVerificationRequired）。用于供应商查看自己渠道 key 等
// "只要求开过 2FA" 的受控动作；未开启则返回 403 + TWO_FA_NOT_ENABLED，供前端弹窗引导去设置。
func RequireTwoFAEnabled() gin.HandlerFunc {
	return func(c *gin.Context) {
		userId := c.GetInt("id")
		if userId == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "未登录",
			})
			c.Abort()
			return
		}
		if model.IsTwoFAEnabled(userId) {
			c.Next()
			return
		}
		if _, err := model.GetPasskeyByUserID(userId); err == nil {
			c.Next()
			return
		}
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "您需要先启用两步验证或 Passkey 才能执行此操作",
			"code":    "TWO_FA_NOT_ENABLED",
		})
		c.Abort()
	}
}

func clearSecureVerificationSession(session sessions.Session) {
	session.Delete(SecureVerificationSessionKey)
	session.Delete(secureVerificationMethodSessionKey)
	_ = session.Save()
}

// OptionalSecureVerification 可选的安全验证中间件
// 如果用户已验证，则在 context 中设置标记，但不阻止请求继续
// 用于某些需要区分是否已验证的场景
func OptionalSecureVerification() gin.HandlerFunc {
	return func(c *gin.Context) {
		userId := c.GetInt("id")
		if userId == 0 {
			c.Set("secure_verified", false)
			c.Next()
			return
		}

		session := sessions.Default(c)
		verifiedAtRaw := session.Get(SecureVerificationSessionKey)

		if verifiedAtRaw == nil {
			c.Set("secure_verified", false)
			c.Next()
			return
		}

		verifiedAt, ok := verifiedAtRaw.(int64)
		if !ok {
			c.Set("secure_verified", false)
			c.Next()
			return
		}

		elapsed := time.Now().Unix() - verifiedAt
		if elapsed >= SecureVerificationTimeout {
			clearSecureVerificationSession(session)
			c.Set("secure_verified", false)
			c.Next()
			return
		}

		c.Set("secure_verified", true)
		c.Set("secure_verified_at", verifiedAt)
		c.Next()
	}
}

// ClearSecureVerification 清除安全验证状态
// 用于用户登出或需要强制重新验证的场景
func ClearSecureVerification(c *gin.Context) {
	session := sessions.Default(c)
	clearSecureVerificationSession(session)
}
