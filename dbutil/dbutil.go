package dbutil

import "github.com/jackc/pgx"
import "fmt"
import "strings"

type DBI interface {
	Exec(sql string, args ...interface{}) (pgx.CommandTag, error)
	Query(sql string, args ...interface{}) (*pgx.Rows, error)
	QueryRow(sql string, args ...interface{}) *pgx.Row
}

func makeInsertPairs(args ...interface{}) (keystr, placeholderstr string, values []interface{}) {
	isKey := true
	var keys []string
	var placeholders []string
	no := 1
	for _, arg := range args {
		if isKey {
			keys = append(keys, "\""+arg.(string)+"\"")
			placeholders = append(placeholders, fmt.Sprintf("$%d", no))
			no++
		} else {
			values = append(values, arg)
		}
		isKey = !isKey
	}

	keystr = strings.Join(keys, ",")
	placeholderstr = strings.Join(placeholders, ",")
	return
}

func InsertKVR(dbi DBI, table, returncol string, args ...interface{}) *pgx.Row {
	keystr, placeholderstr, values := makeInsertPairs(args...)

	sql := fmt.Sprintf("INSERT INTO \"%s\" (%s) VALUES (%s) RETURNING %s", table, keystr, placeholderstr, returncol)
	return dbi.QueryRow(sql, values...)
}

func InsertKV(dbi DBI, table string, args ...interface{}) (pgx.CommandTag, error) {
	keystr, placeholderstr, values := makeInsertPairs(args...)

	sql := fmt.Sprintf("INSERT INTO \"%s\" (%s) VALUES (%s)", table, keystr, placeholderstr)
	return dbi.Exec(sql, values...)
}

type Set map[string]interface{}
type Where map[string]interface{}

func UpdateKV(dbi DBI, table string, s Set, w Where) (pgx.CommandTag, error) {
	var parts []string
	var values []interface{}
	no := 1
	for k, v := range s {
		parts = append(parts, fmt.Sprintf("%s=$%d", k, no))
		values = append(values, v)
		no++
	}

	var wparts []string
	for k, v := range w {
		wparts = append(wparts, fmt.Sprintf("%s=$%d", k, no))
		values = append(values, v)
		no++
	}

	var sql string
	if len(wparts) > 0 {
		sql = fmt.Sprintf("UPDATE \"%s\" SET %s WHERE %s", table, strings.Join(parts, ","), strings.Join(wparts, ","))
	} else {
		panic("not doing unilateral update")
		sql = fmt.Sprintf("UPDATE \"%s\" SET %s", table, strings.Join(parts, ","))
	}

	return dbi.Exec(sql, values...)
}

func IsUniqueViolation(err error) bool {
	if pgerr, ok := err.(pgx.PgError); ok {
		return pgerr.Code == "23505"
	}

	return false
}
