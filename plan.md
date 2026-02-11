Build MUD Platform

1. Scripting Engine (Shopify/go-lua)
  - Interface to work with -- agnostic of underlying library
  - Load arbitrary go types
  - Ideally have some kind of REPL
  - Plugin architecture
    - Define new entities
    - Define hooks into various lifecycle functions
2. Data storage (sqlite3)
3. Web interface (chi, or one of those, or native, htmx)
4. Game machanics
  - ECS, or simple entity system?
5. Editing/Admin UI
  - Building rooms, building mobs, scripting
