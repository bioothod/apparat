package auth

import (
	_ "github.com/go-sql-driver/mysql"
	"database/sql"
	"fmt"
	"time"
)

type AuthCtl struct {
	db		*sql.DB
}

func NewAuthCtl(dbtype, dbparams string) (*AuthCtl, error) {
	db, err := sql.Open(dbtype, dbparams)
	if err != nil {
		return nil, fmt.Errorf("could not open db: %s, params: %s: %v", dbtype, dbparams, err)
	}

	ctl := &AuthCtl {
		db:		db,
	}

	return ctl, nil
}

func (ctl *AuthCtl) Close() {
	ctl.db.Close()
}

type Mailbox struct {
	Username		string		`json:"username"`
	Password		string		`json:"password"`
	Realname		string		`json:"realname"`
	Email			string		`json:"email"`
	Created			time.Time	`json:"-"`
	ID			int64		`json:"-"`
}

func (mbox *Mailbox) String() string {
	return fmt.Sprintf("username: %s, realname: %s, email: %s, created: '%s', uid: %d",
		mbox.Username, mbox.Realname, mbox.Email, mbox.Created.String(), mbox.ID)
}

func (ctl *AuthCtl) NewUser(mbox *Mailbox) error {
	mbox.Created = time.Now()

	res, err := ctl.db.Exec("INSERT INTO users SET username=?,password=?,realname=?,email=?,created=?",
		mbox.Username, mbox.Password, mbox.Realname, mbox.Email, mbox.Created)
	if err != nil {
		return fmt.Errorf("could not insert new user: %s: %v", mbox.String(), err)
	}

	mbox.ID, err = res.LastInsertId()
	if err != nil {
		return fmt.Errorf("could not get ID for user: %s: %v", mbox.String(), err)
	}

	return nil
}

func (ctl *AuthCtl) GetUser(mbox *Mailbox) error {
	rows, err := ctl.db.Query("SELECT * FROM users WHERE username=?", mbox.Username)
	if err != nil {
		return fmt.Errorf("could not read userinfo for user: %s: %v", mbox.Username, err)
	}
	defer rows.Close()

	for rows.Next() {
		var username, password string

		err = rows.Scan(&mbox.ID, &username, &password, &mbox.Realname, &mbox.Email, &mbox.Created)
		if err != nil {
			return fmt.Errorf("database schema mismatch: %v", err)
		}

		if password != mbox.Password || username != mbox.Username {
			return fmt.Errorf("username or password mismatch");
		} else {
			return nil
		}
	}

	err = rows.Err()
	if err != nil {
		return fmt.Errorf("could not scan database: %v", err)
	}

	return fmt.Errorf("there is no user %s", mbox.Username)
}

func (ctl *AuthCtl) UpdateUser(mbox *Mailbox) error {
	_, err := ctl.db.Exec("UPDATE users SET password=?,realname=?,email=? WHERE uid=?",
		mbox.Password, mbox.Realname, mbox.Email, mbox.ID)
	if err != nil {
		return fmt.Errorf("could not update user: %s: %v", mbox.String(), err)
	}

	return nil
}

func (ctl *AuthCtl) Ping() error {
	return ctl.db.Ping()
}
