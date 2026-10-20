import pathlib
p = pathlib.Path(r'E:\Develop\resin\p3_patch_app.py')
s = p.read_text(encoding='utf-8')
s = s.replace('''log.Println("Probe concurrency governor started")''')
n\n# stop governor
rep(''', '''log.Println("Probe concurrency governor started")''')

# stop governor
rep(''')
p.write_text(s, encoding='utf-8')
print('OK fixed script')
