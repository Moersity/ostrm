package app

import (
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"time"
)

func randomID() string {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}
func (a *App) initSecret() error {
	m, e := a.Store.Setting("auth", nil)
	if e != nil {
		return e
	}
	s := str(m, "secret")
	if s == "" {
		s = randomID()
		if e = a.Store.Set("auth", Object{"secret": s}); e != nil {
			return e
		}
	}
	a.secret = []byte(s)
	return nil
}
func (a *App) hasUser() (bool, error) {
	var n int
	e := a.Store.DB.QueryRow("SELECT count(*) FROM admin").Scan(&n)
	return n > 0, e
}
func (a *App) register(m Object) error {
	if e := require(m, "username", "password"); e != nil {
		return e
	}
	if len(str(m, "password")) < 6 {
		return errors.New("密码至少6个字符")
	}
	if len(str(m, "username")) > 200 {
		return errors.New("用户名过长")
	}
	h, e := bcrypt.GenerateFromPassword([]byte(str(m, "password")), bcrypt.DefaultCost)
	if e != nil {
		return e
	}
	_, e = a.Store.DB.Exec("INSERT INTO admin(id,username,password) VALUES(1,?,?)", str(m, "username"), string(h))
	if e != nil {
		return errors.New("用户已存在")
	}
	return nil
}
func (a *App) password(user, pwd string) error {
	var u, h string
	e := a.Store.DB.QueryRow("SELECT username,password FROM admin WHERE id=1").Scan(&u, &h)
	if e != nil && !errors.Is(e, sql.ErrNoRows) {
		return e
	}
	if e != nil || u != user {
		return errors.New("用户名或密码错误")
	}
	if len(h) == 32 {
		sum := md5.Sum([]byte(pwd))
		if subtle.ConstantTimeCompare([]byte(hex.EncodeToString(sum[:])), []byte(h)) != 1 {
			return errors.New("用户名或密码错误")
		}
		b, e := bcrypt.GenerateFromPassword([]byte(pwd), bcrypt.DefaultCost)
		if e != nil {
			return e
		}
		_, e = a.Store.DB.Exec("UPDATE admin SET password=? WHERE id=1", string(b))
		return e
	}
	if bcrypt.CompareHashAndPassword([]byte(h), []byte(pwd)) != nil {
		return errors.New("用户名或密码错误")
	}
	return nil
}
func (a *App) token(user string) (Object, error) {
	id := randomID()
	expires := time.Now().Add(14 * 24 * time.Hour)
	c := jwt.RegisteredClaims{Subject: user, ID: id, Issuer: "ostrm", IssuedAt: jwt.NewNumericDate(time.Now()), ExpiresAt: jwt.NewNumericDate(expires)}
	t, e := jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(a.secret)
	if e != nil {
		return nil, e
	}
	_, e = a.Store.DB.Exec("INSERT INTO sessions(id,username,expires) VALUES(?,?,?)", id, user, expires.Unix())
	return Object{"username": user, "token": t, "expiresAt": expires.UnixMilli()}, e
}
func (a *App) verify(token string) (*jwt.RegisteredClaims, error) {
	c := &jwt.RegisteredClaims{}
	t, e := jwt.ParseWithClaims(token, c, func(t *jwt.Token) (any, error) { return a.secret, nil }, jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer("ostrm"), jwt.WithExpirationRequired())
	if e != nil || !t.Valid {
		return nil, errors.New("未授权访问")
	}
	var user string
	e = a.Store.DB.QueryRow("SELECT username FROM sessions WHERE id=? AND expires>?", c.ID, time.Now().Unix()).Scan(&user)
	if e != nil || user != c.Subject {
		return nil, errors.New("会话已失效")
	}
	return c, nil
}
func (a *App) changePassword(user string, m Object) error {
	if e := a.password(user, str(m, "oldPassword")); e != nil {
		return e
	}
	p := str(m, "newPassword")
	if len(p) < 6 {
		return errors.New("密码至少6个字符")
	}
	b, e := bcrypt.GenerateFromPassword([]byte(p), bcrypt.DefaultCost)
	if e != nil {
		return e
	}
	tx, e := a.Store.DB.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if _, e = tx.Exec("UPDATE admin SET password=? WHERE id=1", string(b)); e != nil {
		return e
	}
	if _, e = tx.Exec("DELETE FROM sessions"); e != nil {
		return e
	}
	return tx.Commit()
}
func hashBytes(b []byte) string { s := sha256.Sum256(b); return fmt.Sprintf("%x", s[:]) }
