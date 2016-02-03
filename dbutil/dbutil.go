package dbutil

import (
	"fmt"
	"github.com/hlandau/xlog"
	"github.com/jackc/pgx"
	"strings"
)

var log, Log = xlog.New("dbutil")

type DBI interface {
	Exec(sql string, args ...interface{}) (pgx.CommandTag, error)
	Query(sql string, args ...interface{}) (*pgx.Rows, error)
	QueryRow(sql string, args ...interface{}) *pgx.Row
}

func makeInsertPairs(extras []string, args ...interface{}) (keystr, placeholderstr string, values []interface{}) {
	isKey := true
	var keys = make([]string, len(extras), len(args)/2+len(extras))
	var placeholders = make([]string, 0, len(args)/2+len(extras))
	values = make([]interface{}, len(extras), len(args)/2+len(extras))
	no := 1
	for i := 0; i < len(extras); i++ {
		keys[i] = extras[i]
		placeholders = append(placeholders, fmt.Sprintf("$%d", no))
		no++
	}
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

var nodeColumns = map[string]struct{}{
	"shortname": struct{}{},
	"longname":  struct{}{},
}

func InsertEdge(dbi DBI, node1ID, node2ID int64, edgeType string, args ...interface{}) error {
	keystr, placeholderstr, values := makeInsertPairs([]string{"node1_id", "node2_id", "type"}, args...)
	values[0] = node1ID
	values[1] = node2ID
	values[2] = edgeType
	sql := fmt.Sprintf("INSERT INTO edge (%s) VALUES (%s)", keystr, placeholderstr)
	_, err := dbi.Exec(sql, values...)
	return err
}

func InsertNode(dbi DBI, nodeType string, args ...interface{}) (int64, error) {
	isKey := true
	nkeys := []string{"type"}
	tkeys := []string{"node_id"}
	nvalues := []interface{}{nodeType}
	nplaceholders := []string{"$1"}
	tplaceholders := []string{"$1"}
	tvalues := []interface{}{int64(0)}
	no := 2
	to := 2
	isNodeColumn := false
	for _, arg := range args {
		if isKey {
			k := arg.(string)
			_, isNodeColumn = nodeColumns[k]
			if isNodeColumn {
				nkeys = append(nkeys, `"`+k+`"`)
				nplaceholders = append(nplaceholders, fmt.Sprintf("$%d", no))
				no++
			} else {
				tkeys = append(tkeys, `"`+k+`"`)
				tplaceholders = append(tplaceholders, fmt.Sprintf("$%d", to))
				to++
			}
		} else {
			if isNodeColumn {
				nvalues = append(nvalues, arg)
			} else {
				tvalues = append(tvalues, arg)
			}
		}
		isKey = !isKey
	}

	nkeystr := strings.Join(nkeys, ",")
	nplaceholderstr := strings.Join(nplaceholders, ",")
	tkeystr := strings.Join(tkeys, ",")
	tplaceholderstr := strings.Join(tplaceholders, ",")

	sql := fmt.Sprintf(`INSERT INTO node (%s) VALUES (%s) RETURNING node_id`, nkeystr, nplaceholderstr)
	log.Debugf("node %s", sql)
	var nodeID int64
	err := dbi.QueryRow(sql, nvalues...).Scan(&nodeID)
	if err != nil {
		return 0, err
	}

	tvalues[0] = nodeID
	sql = fmt.Sprintf(`INSERT INTO "%s" (%s) VALUES (%s)`, "n_"+nodeType, tkeystr, tplaceholderstr)
	log.Debugf("node %s", sql)
	_, err = dbi.Exec(sql, tvalues...)
	if err != nil {
		return 0, err
	}

	return nodeID, nil
}

func InsertKVR(dbi DBI, table, returncol string, args ...interface{}) *pgx.Row {
	keystr, placeholderstr, values := makeInsertPairs(nil, args...)

	sql := fmt.Sprintf("INSERT INTO \"%s\" (%s) VALUES (%s) RETURNING %s", table, keystr, placeholderstr, returncol)
	return dbi.QueryRow(sql, values...)
}

func InsertKV(dbi DBI, table string, args ...interface{}) (pgx.CommandTag, error) {
	keystr, placeholderstr, values := makeInsertPairs(nil, args...)

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
