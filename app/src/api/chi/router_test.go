package chi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRoutingContextRoutePattern(t *testing.T) {
	t.Log("проверяем RoutePattern для пустого контекста")
	assert.Equal(t, "", (&RoutingContext{}).RoutePattern())
	assert.Equal(t, "", (*RoutingContext)(nil).RoutePattern())

	t.Log("устанавливаем маршрут и проверяем результат")
	rc := &RoutingContext{routePattern: "/items"}
	assert.Equal(t, "/items", rc.RoutePattern())
}

func TestRouteContext(t *testing.T) {
	t.Log("проверяем отсутствие контекста маршрута")
	ctx := context.Background()
	assert.Nil(t, RouteContext(ctx))

	t.Log("сохраняем и читаем контекст маршрута")
	rc := &RoutingContext{routePattern: "/"}
	ctx = context.WithValue(ctx, routeCtxKey, rc)
	assert.Equal(t, rc, RouteContext(ctx))
}

func TestMuxRegistersAndServesRoutes(t *testing.T) {
	t.Log("создаём роутер и регистрируем middleware")
	mux := NewRouter()
	called := false
	mux.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			next.ServeHTTP(w, r)
		})
	})

	mux.Get("/hello", func(w http.ResponseWriter, r *http.Request) {
		if rc := RouteContext(r.Context()); rc != nil {
			w.Header().Set("X-Route", rc.RoutePattern())
		}
		w.WriteHeader(http.StatusCreated)
	})

	t.Log("выполняем запрос к маршруту")
	req := httptest.NewRequest(http.MethodGet, "/hello", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	t.Log("проверяем, что middleware и обработчик вызвались")
	assert.True(t, called)
	assert.Equal(t, http.StatusCreated, rr.Code)
	assert.Equal(t, "/hello", rr.Header().Get("X-Route"))
}

func TestMethodRegistersHandlers(t *testing.T) {
	t.Log("регистрируем обработчик POST")
	mux := NewRouter()
	mux.Method("post", "/submit", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})

	t.Log("проверяем статус ответа")
	req := httptest.NewRequest(http.MethodPost, "/submit", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusAccepted, rr.Code)
}

func TestServeHTTPHandlesNotFoundAndMethodNotAllowed(t *testing.T) {
	t.Log("настраиваем маршруты и обработчики ошибок")
	mux := NewRouter()
	mux.Get("/onlyget", func(http.ResponseWriter, *http.Request) {})

	mux.NotFound(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
	mux.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Allow", "GET")
		w.WriteHeader(http.StatusMethodNotAllowed)
	})

	t.Log("проверяем поведение при неверном методе")
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/onlyget", nil)
	mux.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
	assert.Equal(t, "GET", rr.Header().Get("Allow"))

	t.Log("проверяем обработку отсутствующего маршрута")
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/missing", nil)
	mux.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusTeapot, rr.Code)
}

func TestServeHTTPHandlesNilRequest(t *testing.T) {
	t.Log("проверяем поведение при пустом запросе")
	mux := NewRouter()
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, nil)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestAllowedMethodsSorting(t *testing.T) {
	t.Log("регистрируем несколько методов для одного маршрута")
	mux := NewRouter()
	mux.Get("/item", func(http.ResponseWriter, *http.Request) {})
	mux.Method(http.MethodPost, "/item", func(http.ResponseWriter, *http.Request) {})

	t.Log("убеждаемся, что методы отсортированы")
	methods := mux.allowedMethods("/item")
	assert.Equal(t, "GET, POST", methods)
}
