VERSION = "1.0.0"
local uutil = import("micro/util")
local utf8 = import("utf8")
local autoclosePairs = {"\"\"", "''", "``", "()", "{}", "[]"}
local autoNewlinePairs = {"()", "{}", "[]"}
function charAt(str, i)
    return uutil.RuneAt(str, i-1)
end
function onRune(bp, r)
    for i = 1, #autoclosePairs do
        if r == charAt(autoclosePairs[i], 2) then
            local curLine = bp.Buf:Line(bp.Cursor.Y)
            if charAt(curLine, bp.Cursor.X+1) == charAt(autoclosePairs[i], 2) then
                bp:Backspace()
                bp:CursorRight()
                break
            end
            if bp.Cursor.X > 1 and (uutil.IsWordChar(charAt(curLine, bp.Cursor.X-1)) or charAt(curLine, bp.Cursor.X-1) == charAt(autoclosePairs[i], 1)) then
                break
            end
        end
        if r == charAt(autoclosePairs[i], 1) then
            local curLine = bp.Buf:Line(bp.Cursor.Y)
            if bp.Cursor.X == uutil.CharacterCountInString(curLine) or not uutil.IsWordChar(charAt(curLine, bp.Cursor.X+1)) then
                bp.Buf:Insert(-bp.Cursor.Loc, charAt(autoclosePairs[i], 2))
                bp:CursorLeft()
                break
            end
        end
    end
    return true
end
function preInsertNewline(bp)
    local curLine = bp.Buf:Line(bp.Cursor.Y)
    local curRune = charAt(curLine, bp.Cursor.X)
    local nextRune = charAt(curLine, bp.Cursor.X+1)
    local ws = uutil.GetLeadingWhitespace(curLine)
    for i = 1, #autoNewlinePairs do
        if curRune == charAt(autoNewlinePairs[i], 1) then
            if nextRune == charAt(autoNewlinePairs[i], 2) then
                bp.Buf:Insert(-bp.Cursor.Loc, "\n" .. ws)
                bp:StartOfLine()
                bp:CursorLeft()
                bp:InsertNewline()
                bp:InsertTab()
                return false
            end
        end
    end
    return true
end
function preBackspace(bp)
    for i = 1, #autoclosePairs do
        local curLine = bp.Buf:Line(bp.Cursor.Y)
        if charAt(curLine, bp.Cursor.X+1) == charAt(autoclosePairs[i], 2) and charAt(curLine, bp.Cursor.X) == charAt(autoclosePairs[i], 1) then
            bp:Delete()
        end
    end
    return true
end