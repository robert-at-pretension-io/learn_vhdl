package main
import (
  "encoding/json"
  "fmt"
  "github.com/robert-at-pretension-io/vhdl-lint/internal/policy"
)
func main(){
  in := policy.Input{}
  b,_ := json.Marshal(in)
  fmt.Println(string(b))
}
