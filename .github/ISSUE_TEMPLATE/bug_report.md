---
name: Bug report
about: Found an issue with nerd? Let us know so we can fix it
title: ''
labels: bug
assignees: ''

---

**Describe the bug**
A clear and concise description of what the bug is.

**To Reproduce**
Steps to reproduce the behavior:
1. Go to '...'
2. Click on '....'
3. Scroll down to '....'
4. See error

**Expected behavior**
A clear and concise description of what you expected to happen.

**Environment**
* nerd:
  * Version:
  * Config (retrieve with `docker inspect nerd --format='{{json .Config.Env}}'` or equivalent):
    ```json
    # Paste your nerd environment variables here.
    # Be sure to scrub any sensitive values
    ```
* Kafka (if present):
  * Version:
  * Config  (retrieve with `grep -v -e "^$" -e "#" server.properties` from the Kafka config dir or equivalent):
    ```
    # Paste your Kafka config here.
    # Be sure to scrub any sensitive values
    ```
* Redis (if present):
  * Version:
  * Config (retrieve with `CONFIG GET *` from a redis-cli prompt):
    ```
    # Paste your Redis config here.
    # Be sure to scrub any sensitive values
    ```
* Elasticsearch (if present):
  * Version:
  * Config (retrieve from `<elasticsearch-host>:9200/_cluster/settings?include_defaults`):
    ```json
    # Paste your Elasticsearch config here.
    # Be sure to scrub any sensitive values
    ```

**Additional context**
Add any other context about the problem here.