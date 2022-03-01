[**gotkit**](https://pkg.go.dev/github.com/diamondburned/gotkit) (pronounced
"got-kit") is a set of useful GTK4 components and functions that will aid in
developing certain specific types of applications.

```
.
├── app               -- Application abstractions
│   ├── locale        -- Localization (l10n)
│   ├── notify        -- Notification helpers
│   ├── prefs         -- App preference registry
│   │   └── kvstate   -- App state registry
│   └── sounds        -- System sound API
├── components
│   ├── actionbutton
│   ├── animations
│   ├── autoscroll
│   ├── dialogs
│   ├── errpopup
│   ├── logui
│   ├── onlineimage
│   ├── prefui
│   └── title
└── gtkutil           -- GTK additions and helper functions
    ├── aggressivegc  -- Aggressive background GC (1min)
    ├── cssutil       -- CSS injection
    ├── imgutil       -- Online/local image APIs
    └── textutil      -- Markup and TextView helpers
```

---

Package licensed under the Mozilla Public License Version 2.0.
