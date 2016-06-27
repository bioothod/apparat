package index

import (
	"github.com/golang/glog"
	_ "github.com/go-sql-driver/mysql"
	"database/sql"
	"fmt"
)

type IndexCtl struct {
	db		*sql.DB
}

func NewIndexCtl(dbtype, dbparams string) (*IndexCtl, error) {
	db, err := sql.Open(dbtype, dbparams)
	if err != nil {
		return nil, fmt.Errorf("could not open db: %s, params: %s: %v", dbtype, dbparams, err)
	}

	ctl := &IndexCtl {
		db:		db,
	}

	return ctl, nil
}

func (ctl *IndexCtl) Close() {
	ctl.db.Close()
}

func (ctl *IndexCtl) Ping() error {
	return ctl.db.Ping()
}

type Name struct {
	Bucket		string		`json:"bucket"`
	Key		string		`json:"key"`
	Name		string		`json:"name"`
}

type Request struct {
	File		Name		`json:"file"`
	Tags		[]string	`json:"tags"`
}

type IndexRequest struct {
	Files		[]Request	`json:"files"`
}

type IndexFiles struct {
	Tags		map[string][]Name	`json:"tags"`
}

type ListRequest struct {
	Tags		[]string		`json:"tags"`
}

type LReply struct {
	Tag		string			`json:"tag"`
	Keys		[]Name			`json:"keys"`
}

type ListReply struct {
	Tags		[]LReply		`json:"tags"`
}


type Indexer struct {
	username		string
	ctl			*IndexCtl
	meta_index		string
}

func (idx *Indexer) index_name(tag string) string {
	return idx.username + ":" + tag;
}

func NewIndexer(username string, ctl *IndexCtl) (*Indexer, error) {
	idx := &Indexer {
		username:		username,
		ctl:			ctl,
	}
	idx.meta_index = idx.index_name("meta")

	err := idx.check_and_create_meta()
	if err != nil {
		glog.Errorf("could not create meta table '%s': %v", idx.meta_index, err)
		return nil, err
	}

	return idx, nil
}

func ReformatIndexRequest(idx *IndexRequest) *IndexFiles {
	ifiles := &IndexFiles {
		Tags:		make(map[string][]Name),
	}

	for _, req := range idx.Files {
		for _, tag := range req.Tags {
			// can do this since appending to nil slice allocates a new one
			ifiles.Tags[tag] = append(ifiles.Tags[tag], req.File)
		}
	}

	return ifiles
}

func (idx *Indexer) check_and_create_meta() error {
	_, err := idx.ctl.db.Exec("CREATE TABLE IF NOT EXISTS `" + idx.meta_index +
		"` (tag VARCHAR(32) NOT NULL PRIMARY KEY) ENGINE=InnoDB DEFAULT CHARSET=UTF8")
	if err != nil {
		return fmt.Errorf("could not create table '%s': %v", idx.meta_index, err)
	}

	return nil
}

func (idx *Indexer) check_and_create_table(tag string) error {
	iname := idx.index_name(tag)

	if iname == idx.meta_index {
		return fmt.Errorf("index '%s' is not allowed", tag)
	}

	rows, err := idx.ctl.db.Query("SELECT `key` FROM `" + iname + "` LIMIT 1")
	if err != nil {
		glog.Errorf("error selecting key from '%s': %v", iname, err)

		_, err = idx.ctl.db.Exec("CREATE TABLE `" + iname + "` (" +
			"`bucket` VARCHAR(32) NOT NULL, " +
			"`key` VARCHAR(255) NOT NULL, " +
			"`name` VARCHAR(255) NOT NULL, " +
			"PRIMARY KEY (`key`)" +
			") ENGINE=InnoDB DEFAULT CHARSET=UTF8")
		if err != nil {
			return fmt.Errorf("could not create table '%s': %v", iname, err)
		}

		_, err = idx.ctl.db.Exec("INSERT INTO `" + idx.meta_index + "` SET tag=?", tag)
		if err != nil {
			return fmt.Errorf("could not insert tag '%s' into '%s' table: %v", tag, idx.meta_index, err)
		}
	} else {
		rows.Close()
	}
	return nil
}

func (idx *Indexer) IndexFiles(tag string, files []Name) error {
	err := idx.check_and_create_table(tag)
	if err != nil {
		glog.Errorf("could not check and create table '%s': %v", tag, err)
		return err
	}

	iname := idx.index_name(tag)

	var values string
	for idx, f := range files {
		fin := ','
		if idx == len(files) - 1 {
			fin = ';'
		}
		values += fmt.Sprintf("('%s', '%s', '%s')%c", f.Bucket, f.Key, f.Name, fin)
	}
	glog.Infof("tag: %s, values: %s", iname, values)
	_, err = idx.ctl.db.Exec("REPLACE INTO `" + iname + "` (`bucket`, `key`, `name`) VALUES " + values)
	if err != nil {
		glog.Errorf("could not insert into tag '%s' values '%s': %v", iname, values, err)
		return fmt.Errorf("could not insert into tag '%s' values '%s': %v", iname, values, err)
	}

	return nil
}

func (idx *Indexer) Index(ireq *IndexRequest) error {
	ifiles := ReformatIndexRequest(ireq)

	for tag, files := range ifiles.Tags {
		err := idx.IndexFiles(tag, files)
		if err != nil {
			glog.Errorf("could not index files: tag: %s, files: %v, error: %v", tag, files, err)
			return err
		}
	}

	return nil
}

func (idx *Indexer) ListIndex(tag string) ([]Name, error) {
	iname := idx.index_name(tag)

	rows, err := idx.ctl.db.Query("SELECT `bucket`,`key`,`name` FROM `" + iname + "`")
	if err != nil {
		return nil, fmt.Errorf("could not read names from tag '%s': %v", iname, err)
	}
	defer rows.Close()

	names := make([]Name, 0)
	for rows.Next() {
		var n Name

		err = rows.Scan(&n.Bucket, &n.Key, &n.Name)
		if err != nil {
			return nil, fmt.Errorf("database schema mismatch: %v", err)
		}

		names = append(names, n)
	}

	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("could not scan database: %v", err)
	}

	return names, nil
}

func (idx *Indexer) ListMeta() (*ListReply, error) {
	rows, err := idx.ctl.db.Query("SELECT `tag` FROM `" + idx.meta_index + "`")
	if err != nil {
		return nil, fmt.Errorf("could not read tags from meta index '%s': %v", idx.meta_index, err)
	}
	defer rows.Close()

	names := make([]Name, 0)
	for rows.Next() {
		var n Name

		err = rows.Scan(&n.Name)
		if err != nil {
			return nil, fmt.Errorf("database schema mismatch: %v", err)
		}

		names = append(names, n)
	}

	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("could not scan database: %v", err)
	}

	reply := &ListReply {
		Tags: []LReply {
			LReply {
				Tag:		"meta",
				Keys:		names,
			},
		},
	}
	return reply, nil
}

func (idx *Indexer) List(lr *ListRequest) (*ListReply, error) {
	reply := &ListReply {
		Tags:		make([]LReply, 0),
	}

	for _, tag := range lr.Tags {
		keys, err := idx.ListIndex(tag)
		if err != nil {
			glog.Errorf("could not list index: tag: %s, error: %v", tag, err)
			return nil, err
		}

		lr := LReply {
			Tag:		tag,
			Keys:		keys,
		}
		reply.Tags = append(reply.Tags, lr)
	}

	return reply, nil
}
