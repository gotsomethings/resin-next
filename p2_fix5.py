import pathlib
p = pathlib.Path(r'E:\Develop\resin\cmd\resin\app_runtime.go')
s = p.read_text(encoding='utf-8')
changed = False
if '"github.com/Resinat/Resin/internal/probe"' not in s:
    s = s.replace('\t"github.com/Resinat/Resin/internal/node"', '\t"github.com/Resinat/Resin/internal/node"\n\t"github.com/Resinat/Resin/internal/platform"\n\t"github.com/Resinat/Resin/internal/probe"', 1)
    changed = True
p.write_text(s, encoding='utf-8')
print('OK imports' if changed else 'imports exist')
