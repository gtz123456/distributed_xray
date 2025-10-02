package controllers

import (
	"go-distributed/web/db"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func VerifyEmail(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.Data(http.StatusBadRequest, "text/html; charset=utf-8", []byte(`
<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<title>Verification Error</title>
<style>
    body { font-family: Arial, sans-serif; background: #f2f2f2; display: flex; justify-content: center; align-items: center; height: 100vh; }
    .container { background: #fff; padding: 40px; border-radius: 10px; box-shadow: 0 4px 10px rgba(0,0,0,0.1); text-align: center; max-width: 400px; }
    h2 { color: #e74c3c; }
    p { color: #333; }
</style>
</head>
<body>
<div class="container">
<h2>Token is required</h2>
<p>Please check your email link and try again.</p>
</div>
</body>
</html>
        `))
		return
	}

	var user db.User
	result := db.DB.First(&user, "verify_token = ?", token)
	if result.Error != nil {
		c.Data(http.StatusBadRequest, "text/html; charset=utf-8", []byte(`
<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<title>Verification Error</title>
<style>
    body { font-family: Arial, sans-serif; background: #f2f2f2; display: flex; justify-content: center; align-items: center; height: 100vh; }
    .container { background: #fff; padding: 40px; border-radius: 10px; box-shadow: 0 4px 10px rgba(0,0,0,0.1); text-align: center; max-width: 400px; }
    h2 { color: #e74c3c; }
    p { color: #333; }
</style>
</head>
<body>
<div class="container">
<h2>Invalid token</h2>
<p>The verification link is invalid. Please sign up again.</p>
</div>
</body>
</html>
        `))
		return
	}

	if user.TokenExpiry.Before(time.Now()) {
		db.DB.Delete(&user)
		c.Data(http.StatusBadRequest, "text/html; charset=utf-8", []byte(`
<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<title>Token Expired</title>
<style>
    body { font-family: Arial, sans-serif; background: #f2f2f2; display: flex; justify-content: center; align-items: center; height: 100vh; }
    .container { background: #fff; padding: 40px; border-radius: 10px; box-shadow: 0 4px 10px rgba(0,0,0,0.1); text-align: center; max-width: 400px; }
    h2 { color: #e74c3c; }
    p { color: #333; }
</style>
</head>
<body>
<div class="container">
<h2>Token expired</h2>
<p>Your verification link has expired. Please sign up again.</p>
</div>
</body>
</html>
        `))
		return
	}

	user.IsVerified = true
	user.VerifyToken = ""
	db.DB.Save(&user)

	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(`
<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<title>Email Verified</title>
<style>
    body { font-family: Arial, sans-serif; background: #f2f2f2; display: flex; justify-content: center; align-items: center; height: 100vh; }
    .container { background: #fff; padding: 40px; border-radius: 10px; box-shadow: 0 4px 10px rgba(0,0,0,0.1); text-align: center; max-width: 400px; }
    h2 { color: #2ecc71; }
    p { color: #333; }
    a { display: inline-block; margin-top: 20px; padding: 10px 20px; color: #fff; background: #3498db; border-radius: 5px; text-decoration: none; }
    a:hover { background: #2980b9; }
</style>
</head>
<body>
<div class="container">
<h2>Email Verified!</h2>
<p>Your email has been successfully verified. You can now log in.</p>
<a href="/login">Go to Login</a>
</div>
</body>
</html>
    `))
}
