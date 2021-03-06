// Package db provides facilities for making
// atomic transactions on a filesystem-based
// database. The databsae itself is a json
// object. The database is read by unmarshaling
// this object into a Go structure, and modifications
// are made by marshaling a modified version
// of the structure back into a json object
// which overwrites the original.
//
// Transactions on the database are atomic in
// the following sense: when the database is
// opened, a file-based lock is acquired which
// grants the caller exclusive access to the
// database until any changes are committed
// (at which point the lock is released). Opening
// the database will fail if the lock cannot be
// acquired (because another transaction is
// in progress). Additionally, if there are errors
// writing the new state of the database, they
// will cause the entire transaction to fail,
// and the database will remain in the state it
// was in before the transaction began. This
// is accomplished by writing a copy of the new
// database state to a temporary file, and then
// moving the temporary file to replace the
// database file. So long as the operating system's
// move functionality is atomic, transactions can
// only succeed or fail - they cannot leave the
// database in a partially-updated state.
package db

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/joshlf/kudos/lib/build"
	"github.com/joshlf/kudos/lib/config"
	"github.com/joshlf/kudos/lib/lockfile"
)

var (
	ErrNeedAbsPath = errors.New("need absolute path")
	ErrLockFailed  = errors.New("could not acquire lock")
)

type db struct {
	// Kudos version and commit hash
	Version string
	Commit  string

	// User's UID
	UID string `json:",omitempty"`

	// Time of save
	Time time.Time

	DB marshaler
}

type marshaler struct {
	v interface{}
}

func (m marshaler) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.v)
}

func (m marshaler) UnmarshalJSON(b []byte) error {
	return json.Unmarshal(b, m.v)
}

func getDBObj(v interface{}) db {
	var d db
	d.Version = build.Version
	d.Commit = build.Commit
	u, err := user.Current()
	if err == nil {
		d.UID = u.Uid
	}
	d.Time = time.Now()
	d.DB = marshaler{v}
	return d
}

// Committer is a function which will take a new
// value for the database and commit it to disk.
// If v is nil, the database is closed with no
// changes written. A Committer can only be called
// once: any subsequent calls will panic.
type Committer func(v interface{}) error

// Open opens the database stored in the directory
// given by path, and unmarshals the contents of the
// database into v (which must be a pointer type).
// The returned Committer can be used to commit any
// changes to the database and close it. path must
// be an absolute path, or Open will return the error
// ErrNeedAbsPath.
//
// Before opening, a lock is acquired on the database;
// this lock is released once changes have been committed.
// If the lock cannot be acquired, Open will return the
// error ErrLockFailed.
func Open(v interface{}, path string) (c Committer, err error) {
	// The path needs to be absolute because the current
	// directory could change between this function returning
	// and the committer being called.
	if !filepath.IsAbs(path) {
		return nil, ErrNeedAbsPath
	}
	lpath := filepath.Join(path, config.DBLockFileName)
	lock, err := lockfile.New(lpath)
	if err != nil {
		panic(fmt.Errorf("db: unexpected error: %v", err))
	}
	// 3 times and 30ms is chosen so that the total
	// time spent waiting will not be a meaningful
	// pause from the perspective of the user (and
	// will also perform well if run in a loop)
	ok, err := lock.TryLockN(3, 30*time.Millisecond)
	if err != nil {
		return nil, fmt.Errorf("acquire lock: %v", err)
	} else if !ok {
		return nil, ErrLockFailed
	}

	// Release the lock if we return an error later on
	defer func() {
		if err != nil {
			lock.Unlock()
		}
	}()

	dbpath := filepath.Join(path, config.DBFileName)
	f, err := os.Open(dbpath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	d := json.NewDecoder(f)
	// we may care about the other fields later,
	// but for now just throw them out
	err = d.Decode(&db{DB: marshaler{v}})
	if err != nil {
		return nil, fmt.Errorf("unmarshal from file: %v", err)
	}

	var done uint32
	c = func(v interface{}) (err error) {
		if !atomic.CompareAndSwapUint32(&done, 0, 1) {
			panic("db: Committer called twice")
		}

		defer func() {
			err2 := lock.Unlock()
			// only return an error from Unlock
			// if we aren't already returning
			// an error
			if err2 != nil && err == nil {
				err = fmt.Errorf("release lock: %v", err2)
			}
		}()

		if v != nil {
			tmppath := filepath.Join(path, config.DBTempFileName)
			f, err := os.Create(tmppath)
			if err != nil {
				return err
			}
			defer f.Close()
			e := json.NewEncoder(f)
			err = e.Encode(getDBObj(v))
			if err != nil {
				return fmt.Errorf("marshal to file: %v", err)
			}
			err = f.Sync()
			if err != nil {
				return fmt.Errorf("marshal to file: %v", err)
			}
			err = os.Rename(tmppath, dbpath)
			if err != nil {
				return fmt.Errorf("atomically update: %v", err)
			}
		}
		return nil
	}
	return c, nil
}

// Read reads the database stored in the directory
// given by path, and unmarshals the contents of the
// database into v (which must be a pointer type).
// No lock is acquired on the database, so the resulting
// Go object cannot be used to commit changes to the
// database. Since updates to the database file itself
// are atomic, this function is safe even though it
// does not acquire a lock.
func Read(v interface{}, path string) error {
	dbpath := filepath.Join(path, config.DBFileName)
	f, err := os.Open(dbpath)
	if err != nil {
		return err
	}
	defer f.Close()

	d := json.NewDecoder(f)
	// we may care about the other fields later,
	// but for now just throw them out
	err = d.Decode(&db{DB: marshaler{v}})
	if err != nil {
		return fmt.Errorf("unmarshal from file: %v", err)
	}
	return nil
}

// TODO(joshlf): Do we care about acquiring a lock in
// this directory when initializing? If we can assume
// that initialization will happen before anyone else
// tries to operate on the database, then yes, but maybe
// we should acquire the lock first just to be safe.
// After all, it would be a very subtle thing to debug
// if a corruption happened.

// Init creates a database in the given directory whose
// initial contents are v. The directory must already
// exist. path must be an absolute path, or Init will
// return ErrNeedAbsPath.
func Init(v interface{}, path string) (err error) {
	// not strictly necessary, but just to be consistent with Open
	if !filepath.IsAbs(path) {
		return ErrNeedAbsPath
	}

	dbpath := filepath.Join(path, config.DBFileName)
	f, err := os.Create(dbpath)
	if err != nil {
		return err
	}
	defer f.Close()

	e := json.NewEncoder(f)
	err = e.Encode(getDBObj(v))
	if err != nil {
		return fmt.Errorf("marshal to file: %v", err)
	}
	err = f.Sync()
	if err != nil {
		return fmt.Errorf("marshal to file: %v", err)
	}
	return nil
}
