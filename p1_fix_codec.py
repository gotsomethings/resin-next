import re, pathlib, sys
p = pathlib.Path(r'E:\Develop\resin\internal\platform\model_codec.go')
s = p.read_text(encoding='utf-8')
if '"time"' in s:
    print('already has time')
else:
    s2 = re.sub(r'import \(\s*"fmt"', 'import (\n\t"fmt"\n\t"time"', s, count=1)
    if s2 == s:
        print('MISS import'); sys.exit(1)
    p.write_text(s2, encoding='utf-8')
    print('OK time import')
