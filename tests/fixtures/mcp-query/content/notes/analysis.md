---
title: "Analysis Notes"
type: note
draft: false
---

# Analysis Notes

Some background text here.

```sql awiki-query id="cf-by-cat"
SELECT category, SUM(amount) AS total FROM trades GROUP BY 1 ORDER BY 1
```

<!-- AWIKI-QUERY-RESULT:cf-by-cat -->
| category | total |
| --- | --- |
<!-- /AWIKI-QUERY-RESULT:cf-by-cat -->
