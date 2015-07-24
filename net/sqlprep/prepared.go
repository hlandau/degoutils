package sqlprep

import "database/sql"
import "reflect"
import "fmt"

func Prepare(p interface{}, db *sql.DB) error {
	t := reflect.TypeOf(p).Elem()
	v := reflect.Indirect(reflect.ValueOf(p))
	nf := t.NumField()
	for i := 0; i < nf; i++ {
		f := t.Field(i)
		if f.Tag == "" {
			continue
		}
		pr, err := db.Prepare(string(f.Tag))
		if err != nil {
			fmt.Printf("error while preparing field %d", i+1)
			return err
		}
		fv := v.Field(i)
		fv.Set(reflect.ValueOf(pr))
	}

	return nil
}

func Close(p interface{}) error {
	t := reflect.TypeOf(p).Elem()
	v := reflect.Indirect(reflect.ValueOf(p))
	nf := t.NumField()
	for i := 0; i < nf; i++ {
		f := t.Field(i)
		if f.Tag == "" {
			continue
		}
		fv := v.Field(i)
		fvi := fv.Interface()
		if fvi != nil {
			if stmt, ok := fvi.(*sql.Stmt); ok {
				stmt.Close()
				fv.Set(reflect.ValueOf((*sql.Stmt)(nil)))
			}
		}
	}
	return nil
}
