package main
import (
  "fmt"
  "reflect"
  "github.com/robert-at-pretension-io/vhdl-lint/internal/policy"
)
func main(){
  t:=reflect.TypeOf(policy.Input{})
  if f, ok := t.FieldByName("Files"); ok {
    fmt.Println(f.Tag.Get("json"))
  } else { fmt.Println("no Files field") }
}
