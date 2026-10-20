import pathlib, sys

p = pathlib.Path(r'E:\Develop\resin\internal\probe\manager.go')
s = p.read_text(encoding='utf-8')

def rep(old, new):
    global s
    if old not in s:
        print('MISS:', repr(old[:70])); sys.exit(1)
    s = s.replace(old, new, 1)

# A. fields
rep('''\tworkerCount int
''','''\tworkerCount int

\t// Dynamic dispatch limit (P3). Workers are pre-created up to workerCount;
\t// the governor lowers effectiveConcurrency without restarting anything.
\tsettingsConcurrency  atomic.Int32
\teffectiveConcurrency atomic.Int32
''')

# B. constant
rep('''\tdefaultQueueCap       = 1024
''','''\tdefaultQueueCap       = 1024

\t// throttleSleep is how long a worker waits when the live limit is reached.
\tthrottleSleep = 25 * time.Millisecond
''')

# C. PushFront
rep('''func (q *probeTaskQueue) Dequeue() (probeTask, bool) {''','''// PushFront puts a throttled task back at the head of its priority lane.
func (q *probeTaskQueue) PushFront(task probeTask, priority probePriority) {
\tq.mu.Lock()
\tdefer q.mu.Unlock()
\tif q.stopped {
\t\treturn
\t}
\tswitch priority {
\tcase probePriorityHigh:
\t\tq.high.items = append([]probeTask{task}, q.high.items[q.high.head:]...)
\t\tq.high.head = 0
\tdefault:
\t\tq.normal.items = append([]probeTask{task}, q.normal.items[q.normal.head:]...)
\t\tq.normal.head = 0
\t}
\tq.notEmpty.Signal()
}

func (q *probeTaskQueue) Dequeue() (probeTask, bool) {''')

# D. NewProbeManager: capture and store
rep('''\treturn &ProbeManager{''','''\tm := &ProbeManager{''')
rep('''\t\tonProbeEvent:                    cfg.OnProbeEvent,
\t}
}''','''\t\tonProbeEvent:                    cfg.OnProbeEvent,
\t}
\tm.settingsConcurrency.Store(int32(conc))
\tm.effectiveConcurrency.Store(int32(conc))
\treturn m
}''')

# E. setters/getters
rep('''func (m *ProbeManager) SetOnProbeEvent(fn func(kind string)) {
\tm.onProbeEvent = fn
}
''','''func (m *ProbeManager) SetOnProbeEvent(fn func(kind string)) {
\tm.onProbeEvent = fn
}

// SettingsConcurrency is the configured ceiling.
func (m *ProbeManager) SettingsConcurrency() int {
\treturn int(m.settingsConcurrency.Load())
}

// EffectiveConcurrency is the currently enforced dispatch limit.
func (m *ProbeManager) EffectiveConcurrency() int {
\treturn int(m.effectiveConcurrency.Load())
}

// SetSettingsConcurrency updates the configured ceiling (hot, no restart).
func (m *ProbeManager) SetSettingsConcurrency(n int) {
\tif n < 1 {
\t\tn = 1
\t}
\tif n > 10000 {
\t\tn = 10000
\t}
\tprev := int(m.settingsConcurrency.Swap(int32(n)))
\tif m.effectiveConcurrency.Load() >= int32(prev) || m.effectiveConcurrency.Load() < 1 {
\t\tm.effectiveConcurrency.Store(int32(n))
\t}
}

// SetEffectiveConcurrency pins the live dispatch limit.
func (m *ProbeManager) SetEffectiveConcurrency(n int) {
\tsettings := int(m.settingsConcurrency.Load())
\tif n < 1 {
\t\tn = 1
\t}
\tif settings > 0 && n > settings {
\t\tn = settings
\t}
\tm.effectiveConcurrency.Store(int32(n))
}
''')

# F. worker throttle
rep('''\t\tstate, ok := m.markTaskRunning(task.key)''','''\t\t// Dynamic throttling: park the task at the front of its lane while the
\t\tlive limit is saturated.
\t\tif int(m.effectiveConcurrency.Load()) <= m.runningCount() {
\t\t\tm.taskQueue.PushFront(task, probePriorityNormal)
\t\t\tselect {
\t\t\tcase <-m.stopCh:
\t\t\t\treturn
\t\t\tcase <-time.After(throttleSleep):
\t\t\t}
\t\t\tcontinue
\t\t}

\t\tstate, ok := m.markTaskRunning(task.key)''')

# G. runningCount
s += '''
func (m *ProbeManager) runningCount() int {
\tcount := 0
\tm.taskStates.Range(func(_ probeTaskKey, st *probeTaskState) bool {
\t\tif st.flags.Load()&taskFlagRunning != 0 {
\t\t\tcount++
\t\t}
\t\treturn true
\t})
\treturn count
}
'''

p.write_text(s, encoding='utf-8')
print('OK manager P3')
