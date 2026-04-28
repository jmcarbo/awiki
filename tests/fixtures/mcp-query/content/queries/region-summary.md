---
title: "Region Summary"
type: query
out: region-summary-out
sources: [trades, regions]
privacy: private
draft: false
---

# Region Summary

```sql
SELECT r.name, SUM(t.amount) FROM trades t JOIN regions r ON t.region = r.id GROUP BY 1
```
