package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func BenchmarkScan(b *testing.B) {
	for _, total := range []int{10000, 100000} {
		b.Run(fmt.Sprint(total), func(b *testing.B) {
			a, e := New(Config{DataDir: b.TempDir(), Listen: "127.0.0.1:0"})
			if e != nil {
				b.Fatal(e)
			}
			defer a.Close()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var q Object
				json.NewDecoder(r.Body).Decode(&q)
				page := int(num(q, "page"))
				files := []Object{}
				for i := (page - 1) * 500; i < min(page*500, total); i++ {
					files = append(files, Object{"name": fmt.Sprintf("Movie-%06d.mkv", i), "size": 100, "is_dir": false})
				}
				json.NewEncoder(w).Encode(Object{"code": 200, "data": Object{"content": files, "total": total}})
			}))
			defer server.Close()
			c := Object{"baseUrl": server.URL}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				files, e := a.scan(context.Background(), c, "/media")
				if e != nil || len(files) != total {
					b.Fatal(len(files), e)
				}
			}
		})
	}
}
