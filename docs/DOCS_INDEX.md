# 🦅 Creaves Documentation

> Wildlife Sanctuary Management System

---

## 🎯 Start Here

**New to Creaves?** → Start with [README.md](README.md)

**Ready to code?** → Go to [AI_AGENT_GUIDE.md](AI_AGENT_GUIDE.md)

**Need quick reference?** → Open [QUICK_REFERENCE.md](QUICK_REFERENCE.md)

**Understanding domain?** → Read [BUSINESS_LOGIC.md](BUSINESS_LOGIC.md)

---

## 📚 Documentation Index

| Document | Purpose | Read When |
|----------|---------|-----------|
| 📘 [README.md](README.md) | Project overview & getting started | First time setup |
| 📋 [QUICK_REFERENCE.md](QUICK_REFERENCE.md) | Daily cheat sheet | Need quick answers |
| 🏗️ [ARCHITECTURE.md](ARCHITECTURE.md) | Technical design | Understanding structure |
| 💼 [BUSINESS_LOGIC.md](BUSINESS_LOGIC.md) | Domain knowledge | Implementing features |
| 🤖 [AI_AGENT_GUIDE.md](AI_AGENT_GUIDE.md) | AI onboarding | Starting work |
| 🗺️ [INDEX.md](INDEX.md) | Navigation guide | Finding information |
| 📝 [TODO.md](../TODO.md) | Improvement backlog | Planning work |

---

## 🚀 Quick Links

### Essential Commands
```bash
buffalo dev              # Start development server
buffalo test             # Run tests
buffalo pop migrate      # Run migrations
```

### Key URLs
- **Development**: http://localhost:3000
- **Admin Login**: admin / admin
- **Dashboard**: /dashboard
- **Animals**: /animals
- **Feeding**: /feeding

### File Locations
- **Actions**: `actions/` - HTTP handlers
- **Models**: `models/` - Business logic
- **Templates**: `templates/` - HTML views
- **Migrations**: `migrations/` - Database changes

---

## 🎓 Learning Path

### Day 1: Basics
1. Read [README.md](README.md)
2. Read [AI_AGENT_GUIDE.md](AI_AGENT_GUIDE.md) → Quick Start
3. Run the application
4. Explore the UI

### Day 2: Understanding
1. Read [BUSINESS_LOGIC.md](BUSINESS_LOGIC.md)
2. Read [ARCHITECTURE.md](ARCHITECTURE.md)
3. Review [QUICK_REFERENCE.md](QUICK_REFERENCE.md)

### Day 3: First Task
1. Pick task from [TODO.md](../TODO.md)
2. Follow [AI_AGENT_GUIDE.md](AI_AGENT_GUIDE.md)
3. Use [QUICK_REFERENCE.md](QUICK_REFERENCE.md) for patterns

---

## 🔍 Find Information

### I need to...

| Task | Document | Section |
|------|----------|---------|
| Start the app | [README.md](README.md) | Quick Start |
| Add a field | [AI_AGENT_GUIDE.md](AI_AGENT_GUIDE.md) | Common Tasks |
| Fix a bug | [AI_AGENT_GUIDE.md](AI_AGENT_GUIDE.md) | Debugging |
| Understand animals | [BUSINESS_LOGIC.md](BUSINESS_LOGIC.md) | Section 1-2 |
| Understand cares | [BUSINESS_LOGIC.md](BUSINESS_LOGIC.md) | Section 3 |
| Understand treatments | [BUSINESS_LOGIC.md](BUSINESS_LOGIC.md) | Section 4 |
| Understand feeding | [BUSINESS_LOGIC.md](BUSINESS_LOGIC.md) | Section 5 |
| Code patterns | [QUICK_REFERENCE.md](QUICK_REFERENCE.md) | Key Patterns |
| Architecture | [ARCHITECTURE.md](ARCHITECTURE.md) | System Design |

---

## 📊 Project Overview

**Creaves** is a wildlife sanctuary management system for tracking rescued animals from discovery through rehabilitation to release.

### Core Business Process
```
Discovery → Intake → Care/Treatment → Outtake
(Found)   (Admit)  (Rehabilitate)   (Release/Transfer/Death)
```

### Technology Stack
- **Backend**: Go 1.18+ with Buffalo Framework
- **Database**: MySQL 8.4
- **ORM**: Pop v6
- **Frontend**: Bootstrap 4, jQuery, Webpack
- **Templates**: Plush (Buffalo templating)
- **Languages**: English & French

---

## 🆘 Getting Help

### Common Issues

**"no transaction found"**  
→ Check [AI_AGENT_GUIDE.md](AI_AGENT_GUIDE.md) → Common Pitfalls

**CSRF token errors**  
→ Check [QUICK_REFERENCE.md](QUICK_REFERENCE.md) → Common Errors

**N+1 queries**  
→ Check [ARCHITECTURE.md](ARCHITECTURE.md) → Enrichment Pattern

**Translation missing**  
→ Check [QUICK_REFERENCE.md](QUICK_REFERENCE.md) → Internationalization

### Debugging

1. Enable debug mode: `pop.Debug = true`
2. Check logs in terminal
3. Use browser DevTools
4. See [AI_AGENT_GUIDE.md](AI_AGENT_GUIDE.md) → Debugging Techniques

---

## 📝 Documentation Quality

This documentation follows these principles:

✅ **Current** - Reflects actual codebase  
✅ **Specific** - Includes code examples  
✅ **Consistent** - Standardized format  
✅ **Cross-referenced** - Links between docs  
✅ **Actionable** - Clear next steps  
✅ **Searchable** - Topic-based organization  

---

## 🎯 Next Steps

### Choose your path:

**🔰 Beginner**  
→ [README.md](README.md) → Complete project overview

**💻 Developer**  
→ [QUICK_REFERENCE.md](QUICK_REFERENCE.md) → Daily reference

**🤖 AI Agent**  
→ [AI_AGENT_GUIDE.md](AI_AGENT_GUIDE.md) → Quick onboarding

**📈 Architect**  
→ [ARCHITECTURE.md](ARCHITECTURE.md) → System design

**💼 Business User**  
→ [BUSINESS_LOGIC.md](BUSINESS_LOGIC.md) → Domain knowledge

**🔍 Navigator**  
→ [INDEX.md](INDEX.md) → Find specific information

---

## 📞 Support

For questions or issues:
1. Check relevant documentation section
2. Review [INDEX.md](INDEX.md) for cross-references
3. Search [TODO.md](../TODO.md) for known issues
4. Consult [QUICK_REFERENCE.md](QUICK_REFERENCE.md) → Common Errors

---

**Last Updated**: 2025-02-25  
**Version**: 1.0.0  
**Project**: Creaves Wildlife Sanctuary Management System

---

## 📁 All Documentation Files

```
docs/
├── README.md                  # Project overview
├── QUICK_REFERENCE.md         # Daily cheat sheet
├── ARCHITECTURE.md            # Technical design
├── BUSINESS_LOGIC.md          # Domain knowledge
├── AI_AGENT_GUIDE.md          # AI onboarding
├── INDEX.md                   # Navigation guide
├── DOCUMENTATION_SUMMARY.md   # This summary
└── DOCS_INDEX.md              # This file (landing page)
```

**Total**: 8 documentation files  
**Total Size**: ~120 KB  
**Total Lines**: ~5,000+ lines

---

*Happy coding! 🚀*
