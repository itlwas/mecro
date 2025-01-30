VERSION = "1.0.0"
local util = import("micro/util")
local config = import("micro/config")
local buffer = import("micro/buffer")
local ft = {
    actionscript = "// %s", ada = "-- %s", apacheconf = "# %s", arduino = "// %s",
    asciidoc = "// %s", asm = "; %s", awk = "# %s", bat = "REM %s",
    c = "// %s", caddyfile = "# %s", cake = "// %s", clojure = "; %s",
    cmake = "# %s", coffeescript = "# %s", conky = "# %s", cpp = "// %s",
    crontab = "# %s", crystal = "# %s", csharp = "// %s", css = "/* %s */",
    csx = "// %s", cuda = "// %s", cython = "# %s", d = "// %s",
    dart = "// %s", dockerfile = "# %s", dot = "// %s", elixir = "# %s",
    elm = "-- %s", env = "# %s", erb = "<%# %s %>", erlang = "% %s",
    fish = "# %s", forth = "\\ %s", fortran = "! %s", fsharp = "// %s",
    gdscript = "# %s", glsl = "// %s", gnuplot = "# %s", go = "// %s",
    golo = "# %s", gomod = "// %s", graphql = "# %s", groovy = "// %s",
    haml = "-# %s", hare = "// %s", haskell = "-- %s", haxe = "// %s",
    html = "<!-- %s -->", html4 = "<!-- %s -->", html5 = "<!-- %s -->", idris = "-- %s",
    ignore = "# %s", ini = "; %s", inputrc = "# %s", java = "// %s",
    javascript = "// %s", jinja2 = "{# %s #}", json = "// %s", jsonnet = "// %s",
    julia = "# %s", justfile = "# %s", kotlin = "// %s", ledger = "# %s",
    lfe = "; %s", lisp = "; %s", lua = "-- %s", makefile = "# %s",
    markdown = "<!-- %s -->", mpdconf = "# %s", nanorc = "# %s", nftables = "# %s",
    nginx = "# %s", nim = "# %s", nix = "# %s", nu = "# %s",
    objc = "// %s", ocaml = "(* %s *)", octave = "# %s", odin = "// %s",
    openscad = "// %s", pascal = "{ %s }", perl = "# %s", php = "// %s",
    po = "# %s", pony = "// %s", powershell = "# %s", proto = "// %s",
    puppet = "# %s", python2 = "# %s", python3 = "# %s", r = "# %s",
    raku = "# %s", renpy = "# %s", ruby = "# %s", rust = "// %s",
    sage = "# %s", scala = "// %s", sed = "# %s", sh = "# %s",
    smalltalk = "\" %s", solidity = "// %s", sql = "-- %s", svelte = "<!-- %s -->",
    swift = "// %s", systemd = "# %s", tcl = "# %s", terraform = "# %s",
    tex = "% %s", toml = "# %s", twig = "{# %s #}", typescript = "// %s",
    v = "// %s", vala = "// %s", verilog = "// %s", vhdl = "-- %s",
    vi = "\" %s", vue = "<!-- %s -->", xml = "<!-- %s -->", xresources = "! %s",
    yaml = "# %s", yum = "# %s", zig = "// %s", zscript = "// %s",
    zsh = "# %s"
}
local last_ft
function updateCommentType(buf)
    if buf.Settings["commenttype"] == nil or (last_ft ~= buf.Settings["filetype"] and last_ft ~= nil) then
        if ft[buf.Settings["filetype"]] ~= nil then
            buf:SetOptionNative("commenttype", ft[buf.Settings["filetype"]])
        else
            buf:SetOptionNative("commenttype", "# %s")
        end
        last_ft = buf.Settings["filetype"]
    end
end
function isCommented(bp, lineN, commentRegex)
    local line = bp.Buf:Line(lineN)
    local regex = commentRegex:gsub("%s+", "%s*")
    if string.match(line, regex) then
        return true
    end
    return false
end
function commentLine(bp, lineN, indentLen)
    updateCommentType(bp.Buf)
    local line = bp.Buf:Line(lineN)
    local commentType = bp.Buf.Settings["commenttype"]
    local sel = -bp.Cursor.CurSelection
    local curpos = -bp.Cursor.Loc
    local index = string.find(commentType, "%%s") - 1
    local indent = string.sub(line, 1, indentLen)
    local trimmedLine = string.sub(line, indentLen + 1)
    trimmedLine = trimmedLine:gsub("%%", "%%%%")
    local commentedLine = commentType:gsub("%%s", trimmedLine)
    bp.Buf:Replace(buffer.Loc(0, lineN), buffer.Loc(#line, lineN), indent .. commentedLine)
    if bp.Cursor:HasSelection() then
        bp.Cursor.CurSelection[1].Y = sel[1].Y
        bp.Cursor.CurSelection[2].Y = sel[2].Y
        bp.Cursor.CurSelection[1].X = sel[1].X
        bp.Cursor.CurSelection[2].X = sel[2].X
    else
        bp.Cursor.X = curpos.X + index
        bp.Cursor.Y = curpos.Y
    end
    bp.Cursor:Relocate()
    bp.Cursor:StoreVisualX()
end
function uncommentLine(bp, lineN, commentRegex)
    updateCommentType(bp.Buf)
    local line = bp.Buf:Line(lineN)
    local commentType = bp.Buf.Settings["commenttype"]
    local sel = -bp.Cursor.CurSelection
    local curpos = -bp.Cursor.Loc
    local index = string.find(commentType, "%%s") - 1
    if not string.match(line, commentRegex) then
        commentRegex = commentRegex:gsub("%s+", "%s*")
    end
    if string.match(line, commentRegex) then
        local uncommentedLine = string.match(line, commentRegex)
        bp.Buf:Replace(buffer.Loc(0, lineN), buffer.Loc(#line, lineN), util.GetLeadingWhitespace(line) .. uncommentedLine)
        if bp.Cursor:HasSelection() then
            bp.Cursor.CurSelection[1].Y = sel[1].Y
            bp.Cursor.CurSelection[2].Y = sel[2].Y
            bp.Cursor.CurSelection[1].X = sel[1].X
            bp.Cursor.CurSelection[2].X = sel[2].X
        else
            bp.Cursor.X = curpos.X - index
            bp.Cursor.Y = curpos.Y
        end
    end
    bp.Cursor:Relocate()
    bp.Cursor:StoreVisualX()
end
function toggleCommentLine(bp, lineN, commentRegex)
    if isCommented(bp, lineN, commentRegex) then
        uncommentLine(bp, lineN, commentRegex)
    else
        commentLine(bp, lineN, #util.GetLeadingWhitespace(bp.Buf:Line(lineN)))
    end
end
function toggleCommentSelection(bp, startLine, endLine, commentRegex)
    local allComments = true
    for line = startLine, endLine do
        if not isCommented(bp, line, commentRegex) then
            allComments = false
            break
        end
    end
    local indentMin = -1
    if not allComments then
        for line = startLine, endLine do
            local indentLen = #util.GetLeadingWhitespace(bp.Buf:Line(line))
            if indentMin == -1 or indentLen < indentMin then
                indentMin = indentLen
            end
        end
    end
    for line = startLine, endLine do
        if allComments then
            uncommentLine(bp, line, commentRegex)
        else
            commentLine(bp, line, indentMin)
        end
    end
end
function comment(bp)
    updateCommentType(bp.Buf)
    local commentType = bp.Buf.Settings["commenttype"]
    local commentRegex = "^%s*" .. commentType:gsub("%%","%%%%"):gsub("%$","%$"):gsub("%)","%)"):gsub("%(","%("):gsub("%?","%?"):gsub("%*", "%*"):gsub("%-", "%-"):gsub("%.", "%."):gsub("%+", "%+"):gsub("%]", "%]"):gsub("%[", "%["):gsub("%%%%s", "(.*)")
    if bp.Cursor:HasSelection() then
        if bp.Cursor.CurSelection[1]:GreaterThan(-bp.Cursor.CurSelection[2]) then
            local endLine = bp.Cursor.CurSelection[1].Y
            if bp.Cursor.CurSelection[1].X == 0 then
                endLine = endLine - 1
            end
            toggleCommentSelection(bp, bp.Cursor.CurSelection[2].Y, endLine, commentRegex)
        else
            local endLine = bp.Cursor.CurSelection[2].Y
            if bp.Cursor.CurSelection[2].X == 0 then
                endLine = endLine - 1
            end
            toggleCommentSelection(bp, bp.Cursor.CurSelection[1].Y, endLine, commentRegex)
        end
    else
        toggleCommentLine(bp, bp.Cursor.Y, commentRegex)
    end
end
function init()
    config.MakeCommand("comment", comment, config.NoComplete)
    config.TryBindKey("Alt-/", "lua:comment.comment", false)
    config.TryBindKey("CtrlUnderscore", "lua:comment.comment", false)
    config.AddRuntimeFile("comment", config.RTHelp, "help/comment.md")
end