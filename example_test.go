package confix

import (
	"fmt"
	"os"
)

func ExampleNew() {
	dir := os.TempDir()
	type Config struct {
		A string `config:"a" json:"a"`
		B int    `config:"b" json:"b"`
	}
	err := os.WriteFile(dir+"/config.json", []byte(`{"a": "example"}`), 0o666)
	if err != nil {
		panic(err)
	}
	// you can preset default config variables.
	cfg := &Config{
		A: "a",
		B: 1,
	}
	_ = SetConfigDir(dir)
	err = New(cfg, WithSyncingConfigToFiles[Config]())
	if err != nil {
		panic(err)
	}
	fmt.Println(cfg.A, cfg.B)

	data, err := os.ReadFile(dir + "/config.json")
	if err != nil {
		panic(err)
	}
	fmt.Println(string(data))

	// Output:
	// example 1
	// {
	//   "a": "example",
	//   "b": 1
	// }
}
