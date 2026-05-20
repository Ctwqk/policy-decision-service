package rules

type ahoMatcher struct {
	nodes []ahoNode
}

type ahoNode struct {
	next     map[byte]int
	fail     int
	terminal bool
}

func newAhoMatcher(keywords []string) *ahoMatcher {
	matcher := &ahoMatcher{
		nodes: []ahoNode{{next: map[byte]int{}}},
	}
	for _, keyword := range keywords {
		if keyword == "" {
			continue
		}
		node := 0
		for i := 0; i < len(keyword); i++ {
			b := keyword[i]
			next, ok := matcher.nodes[node].next[b]
			if !ok {
				next = len(matcher.nodes)
				matcher.nodes = append(matcher.nodes, ahoNode{next: map[byte]int{}})
				matcher.nodes[node].next[b] = next
			}
			node = next
		}
		matcher.nodes[node].terminal = true
	}
	matcher.buildFailures()
	return matcher
}

func (m *ahoMatcher) buildFailures() {
	queue := make([]int, 0, len(m.nodes))
	for _, child := range m.nodes[0].next {
		m.nodes[child].fail = 0
		queue = append(queue, child)
	}
	for head := 0; head < len(queue); head++ {
		current := queue[head]
		for b, child := range m.nodes[current].next {
			fail := m.nodes[current].fail
			for fail != 0 {
				if fallback, ok := m.nodes[fail].next[b]; ok {
					m.nodes[child].fail = fallback
					break
				}
				fail = m.nodes[fail].fail
			}
			if fail == 0 {
				if fallback, ok := m.nodes[0].next[b]; ok && fallback != child {
					m.nodes[child].fail = fallback
				}
			}
			if m.nodes[m.nodes[child].fail].terminal {
				m.nodes[child].terminal = true
			}
			queue = append(queue, child)
		}
	}
}

func (m *ahoMatcher) match(value string) bool {
	if m == nil || len(m.nodes) == 0 {
		return false
	}
	state := 0
	for i := 0; i < len(value); i++ {
		state = m.nextState(state, value[i])
		if m.nodes[state].terminal {
			return true
		}
	}
	return false
}

func (m *ahoMatcher) nextState(state int, b byte) int {
	for {
		if next, ok := m.nodes[state].next[b]; ok {
			return next
		}
		if state == 0 {
			return 0
		}
		state = m.nodes[state].fail
	}
}
