---
title: "Top Trades"
type: query
out: top-trades-out
sources: [trades]
privacy: private
draft: false
---

# Top Trades

```sql
SELECT category, SUM(amount) AS total FROM trades GROUP BY 1 ORDER BY 1
```
