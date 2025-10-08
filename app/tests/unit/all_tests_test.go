package unit

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRunAllNonIntegrationTests(t *testing.T) {
	if os.Getenv("UNIT_AGGREGATOR_ACTIVE") == "1" {
		t.Log("обнаружен вложенный запуск, пропускаем тест")
		t.Skip("агрегирующий тест уже выполняется")
	}

	t.Log("Шаг 1: подготавливаем корневую директорию проекта и список пакетов")
	root := projectRoot(t)

	listCmd := exec.Command("go", "list", "./...")
	listCmd.Dir = root
	listCmd.Env = os.Environ()

	output, err := listCmd.Output()
	if err != nil {
		t.Fatalf("не удалось получить список пакетов: %v", err)
	}

	t.Log("фильтруем интеграционные и e2e пакеты")
	var packages []string
	for _, pkg := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if pkg == "" {
			continue
		}
		if strings.Contains(pkg, "/tests/integration") || strings.Contains(pkg, "/tests/e2e") {
			continue
		}
		packages = append(packages, pkg)
	}

	if len(packages) == 0 {
		t.Fatal("не найдено пакетов для запуска")
	}

	args := append([]string{"test", "-count=1"}, packages...)
	t.Logf("запускаем go %s", strings.Join(args, " "))

	testCmd := exec.Command("go", args...)
	testCmd.Dir = root
	env := os.Environ()
	env = append(env, "UNIT_AGGREGATOR_ACTIVE=1")
	testCmd.Env = env

	combinedOutput, err := testCmd.CombinedOutput()
	t.Logf("Результат выполнения команды:\n%s", combinedOutput)
	if err != nil {
		t.Fatalf("тесты завершились с ошибкой: %v", err)
	}
}

func projectRoot(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("не удалось определить путь к файлу теста")
	}

	dir := filepath.Dir(filename)
	root := filepath.Clean(filepath.Join(dir, "../../.."))
	t.Logf("Определён корень проекта: %s", root)
	return root
}
