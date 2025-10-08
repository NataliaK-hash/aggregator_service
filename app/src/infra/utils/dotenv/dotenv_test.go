package dotenv

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadAppliesVariablesFromFiles(t *testing.T) {
	t.Log("создаём временный .env файл и загружаем")
	dir := t.TempDir()
	path := filepath.Join(dir, "config.env")
	content := "FOO=bar\nexport BAZ='qux'\n#comment\nEMPTY=\n"
	assert.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	t.Setenv("EMPTY", "existing")

	err := Load(path)
	assert.NoError(t, err)

	t.Log("проверяем, что переменные окружения применились")
	assert.Equal(t, "bar", os.Getenv("FOO"))
	assert.Equal(t, "qux", os.Getenv("BAZ"))
	assert.Equal(t, "existing", os.Getenv("EMPTY"))
}

func TestLoadIgnoresMissingFiles(t *testing.T) {
	t.Log("пытаемся загрузить отсутствующий .env")
	err := Load("does-not-exist.env")
	assert.NoError(t, err)
}

func TestLoadFileReadsLines(t *testing.T) {
	t.Log("проверяем загрузку переменных из файла")
	dir := t.TempDir()
	path := filepath.Join(dir, "vars.env")
	content := "export KEY=value\nSINGLE='quoted'\nDOUBLE=\"spaced value\"\n"
	assert.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	assert.NoError(t, loadFile(path))
	t.Log("проверяем, что значения действительно установлены")
	assert.Equal(t, "value", os.Getenv("KEY"))
	assert.Equal(t, "quoted", os.Getenv("SINGLE"))
	assert.Equal(t, "spaced value", os.Getenv("DOUBLE"))
}

func TestApplyLineBehaviour(t *testing.T) {
	t.Log("проверяем обработку строк с разными форматами")
	t.Setenv("EXISTING", "value")

	assert.NoError(t, applyLine("# comment"))
	assert.NoError(t, applyLine("export NEW=value"))
	assert.Equal(t, "value", os.Getenv("EXISTING"))
	assert.Equal(t, "value", os.Getenv("NEW"))

	assert.NoError(t, applyLine("NOEQUALS"))
	assert.Equal(t, "", os.Getenv("NOEQUALS"))
}
