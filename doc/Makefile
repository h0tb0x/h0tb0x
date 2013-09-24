
DOCFLAGS := -a toc
DOCFLAGS += -a icons
DOCFLAGS += -n

all: h0tb0x.html sync.svg

h0tb0x.html : h0tb0x.asc
	asciidoc $(DOCFLAGS) $< 

sync.svg : sync.uml
	java -jar plantuml.jar -tsvg $<
