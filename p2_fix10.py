import pathlib

# fix test return-value order
p = pathlib.Path(r'E:\Develop\resin\internal\service\platform_verdict_test.go')
s = p.read_text(encoding='utf-8')
s = s.replace('\t\trehab, _ := store.Record("p1", ip, "h1", true, now.Add(time.Duration(10+i)*time.Minute), cfg)',
              '\t\t_, rehab := store.Record("p1", ip, "h1", true, now.Add(time.Duration(10+i)*time.Minute), cfg)', 1)
s = s.replace('\trehab, _ := store.Record("p1", ip, "h1", true, now.Add(13*time.Minute), cfg)',
              '\t_, rehab := store.Record("p1", ip, "h1", true, now.Add(13*time.Minute), cfg)', 1)
s = s.replace('\tif rehab, _ := store.Record("p1", ip, "h1", true, now.Add(4*time.Minute), cfg); rehab {',
              '\tif _, rehab := store.Record("p1", ip, "h1", true, now.Add(4*time.Minute), cfg); rehab {', 1)
p.write_text(s, encoding='utf-8')
print('OK test return order')

# remove debug print
p2 = pathlib.Path(r'E:\Develop\resin\internal\service\platform_verdict.go')
s2 = p2.read_text(encoding='utf-8')
import re
s2 = re.sub(r'\tfmt\.Printf\("DBG pass[^\n]*\n', '', s2)
s2 = s2.replace('import (\n\t"fmt"\n\t"net/netip"', 'import (\n\t"net/netip"', 1)
p2.write_text(s2, encoding='utf-8')
print('OK debug removed')
