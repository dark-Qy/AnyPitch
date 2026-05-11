# AGENTS.md

## Required Reading Before Code Changes

修改代码、文档或协作契约前，按顺序阅读：

1. `AGENTS.md`
2. `ARCHITECTURE.md`
3. `README.md`
4. `FRONTEND.md`
5. `PRODUCT_SENSE.md`
6. `RELIABILITY.md`
7. `SECURITY.md`
8. `rules/` 下对应规则
9. `docs/design-docs/` 与 `docs/product-specs/` 下相关专题

然后回到代码确认真实实现。

## Source Of Truth

- 代码是唯一核心源
- 文档用于导航、原则和边界说明
- 如果文档与代码冲突，以代码为准，并修正文档

## Engineering Role

- 默认按高级工程师标准设计边界
- 保守对待公共 API、SQLite schema 和响应结构
- 前端只负责交互和呈现，不复制后端业务真相
- 完成态必须留下自动化、smoke 和真实 Web 验证证据

## Frontend Skill Rule

涉及页面、组件、布局或视觉时，使用 `frontend-design` 思路先明确：

- `purpose`: 让教练快速管理队伍训练、比赛和战术
- `audience`: 单支球队的教练或队长
- `tone`: 战术板工作台
- `differentiation`: 可拖拽球场战术板是第一识别点
