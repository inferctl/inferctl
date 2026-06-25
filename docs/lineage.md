# inferctl lineage

inferctl is an independent Go implementation. The inference-router idea that
started it came out of tinkering with
[Foxforge](https://github.com/GuideboardLabs/foxforge); inferctl was then
written against its own design specification (the v0.1 contract set) rather
than ported from any existing source.

The reusable CLI-design skill used to scaffold inferctl's agent-facing command
surface now lives at <https://github.com/Ozhiaki/make-cli>. It was developed in
this repository while inferctl's initial contract was taking shape, then moved
out once the guidance became reusable beyond this project.
