VERSION = "3.5.1"
local micro = import("micro")
local config = import("micro/config")
local shell = import("micro/shell")
local buffer = import("micro/buffer")
local os = import("os")
local filepath = import("path/filepath")
local function clear_messenger()
end
local tree_view = nil
local current_dir = os.Getwd()
local highest_visible_indent = 0
local scanlist = {}
local function new_listobj(p, d, o, i)
	return {
		["abspath"] = p,
		["dirmsg"] = d,
		["owner"] = o,
		["indent"] = i,
		["decrease_owner"] = function(self, minus_num)
			self.owner = self.owner - minus_num
		end,
		["increase_owner"] = function(self, plus_num)
			self.owner = self.owner + plus_num
		end
	}
end
local function repeat_str(str, len)
	local string_table = {}
	for i = 1, len do
		string_table[i] = str
	end
	return table.concat(string_table)
end
local function is_dir(path)
	local golib_os = import("os")
	local file_info, stat_error = golib_os.Stat(path)
	if file_info ~= nil then
		return file_info:IsDir()
	else
		micro.InfoBar():Error("Error checking if is dir: ", stat_error)
		return nil
	end
end
local function get_ignored_files(tar_dir)
	local function has_git()
		local git_rp_results = shell.ExecCommand('git  -C "' .. tar_dir .. '" rev-parse --is-inside-work-tree')
		return git_rp_results:match("^true%s*$")
	end
	local readout_results = {}
	if has_git() then
		local git_ls_results =
			shell.ExecCommand('git -C "' .. tar_dir .. '" ls-files . --ignored --exclude-standard --others --directory')
		for split_results in string.gmatch(git_ls_results, "([^\r\n]+)") do
			readout_results[#readout_results + 1] =
				(string.sub(split_results, -1) == "/" and string.sub(split_results, 1, -2) or split_results)
		end
	end
	return readout_results
end
local function get_basename(path)
	if path == nil then
		micro.Log("Bad path passed to get_basename")
		return nil
	else
		local golib_path = import("filepath")
		return golib_path.Base(path)
	end
end
local function is_dotfile(file_name)
	if string.sub(file_name, 1, 1) == "." then
		return true
	else
		return false
	end
end
local function get_scanlist(dir, ownership, indent_n)
	local golib_ioutil = import("ioutil")
	local dir_scan, scan_error = golib_ioutil.ReadDir(dir)
	if dir_scan == nil then
		micro.InfoBar():Error("Error scanning dir: ", scan_error)
		return nil
	end
	local results = {}
	local files = {}
	local function get_results_object(file_name)
		local abs_path = filepath.Join(dir, file_name)
		local dirmsg = (is_dir(abs_path) and "+" or "")
		return new_listobj(abs_path, dirmsg, ownership, indent_n)
	end
	local show_dotfiles = config.GetGlobalOption("filemanager.showdotfiles")
	local show_ignored = config.GetGlobalOption("filemanager.showignored")
	local folders_first = config.GetGlobalOption("filemanager.foldersfirst")
	local ignored_files = (not show_ignored and get_ignored_files(dir) or {})
	local function is_ignored_file(filename)
		for i = 1, #ignored_files do
			if ignored_files[i] == filename then
				return true
			end
		end
		return false
	end
	local filename
	for i = 1, #dir_scan do
		local showfile = true
		filename = dir_scan[i]:Name()
		if not show_dotfiles and is_dotfile(filename) then
			showfile = false
		end
		if not show_ignored and is_ignored_file(filename) then
			showfile = false
		end
		if showfile then
			if folders_first and not is_dir(filepath.Join(dir, filename)) then
				files[#files + 1] = get_results_object(filename)
			else
				results[#results + 1] = get_results_object(filename)
			end
		end
	end
	if #files > 0 then
		for i = 1, #files do
			results[#results + 1] = files[i]
		end
	end
	return results
end
local function get_safe_y(optional_y)
	local y = 0
	if optional_y == nil then
		optional_y = tree_view.Cursor.Loc.Y
	end
	if optional_y > 2 then
		y = tree_view.Cursor.Loc.Y - 2
	end
	return y
end
local function dirname_and_join(path, join_name)
	local leading_path = filepath.Dir(path)
	return filepath.Join(leading_path, join_name)
end
local function select_line(last_y)
	if last_y ~= nil then
		if last_y > 1 then
			tree_view.Cursor.Loc.Y = last_y
		end
	elseif tree_view.Cursor.Loc.Y < 2 then
		tree_view.Cursor.Loc.Y = 2
	end
	tree_view.Cursor:Relocate()
	tree_view:Center()
	tree_view.Cursor:SelectLine()
end
local function scanlist_is_empty()
	if next(scanlist) == nil then
		return true
	else
		return false
	end
end
local function refresh_view()
	clear_messenger()
	if tree_view:GetView().Width < 30 then
		tree_view:ResizePane(30)
	end
	tree_view.Buf.EventHandler:Remove(tree_view.Buf:Start(), tree_view.Buf:End())
	tree_view.Buf.EventHandler:Insert(buffer.Loc(0, 0), current_dir .. "\n")
	tree_view.Buf.EventHandler:Insert(buffer.Loc(0, 1), repeat_str("─", tree_view:GetView().Width) .. "\n")
	tree_view.Buf.EventHandler:Insert(buffer.Loc(0, 2), (#scanlist > 0 and "..\n" or ".."))
	local display_content
	for i = 1, #scanlist do
		if scanlist[i].dirmsg ~= "" then
			display_content = scanlist[i].dirmsg .. " " .. get_basename(scanlist[i].abspath) .. "/"
		else
			display_content = "  " .. get_basename(scanlist[i].abspath)
		end
		if scanlist[i].owner > 0 then
			display_content = repeat_str(" ", 2 * scanlist[i].indent) .. display_content
		end
		if i < #scanlist then
			display_content = display_content .. "\n"
		end
		tree_view.Buf.EventHandler:Insert(buffer.Loc(0, i + 2), display_content)
	end
    tree_view:Tab():Resize()
end
local function move_cursor_top()
	tree_view.Cursor.Loc.Y = 2
	select_line()
end
local function refresh_and_select()
	local last_y = tree_view.Cursor.Loc.Y
	refresh_view()
	select_line(last_y)
end
local function compress_target(y, delete_y)
	if y == 0 or scanlist_is_empty() then
		return
	end
	if scanlist[y].dirmsg == "-" then
		local target_index, delete_index
		local delete_under = {[1] = y}
		local new_table = {}
		local del_count = 0
		for i = 1, #scanlist do
			delete_index = false
			if i ~= y then
				for x = 1, #delete_under do
					if scanlist[i].owner == delete_under[x] then
						delete_index = true
						del_count = del_count + 1
						if scanlist[i].dirmsg == "-" then
							delete_under[#delete_under + 1] = i
						end
						if scanlist[i].indent == highest_visible_indent and scanlist[i].indent > 0 then
							highest_visible_indent = highest_visible_indent - 1
						end
						break
					end
				end
			end
			if not delete_index then
				new_table[#new_table + 1] = scanlist[i]
			end
		end
		scanlist = new_table
		if del_count > 0 then
			for i = y + 1, #scanlist do
				if scanlist[i].owner > y then
					scanlist[i]:decrease_owner(del_count)
				end
			end
		end
		if not delete_y then
			scanlist[y].dirmsg = "+"
		end
	elseif config.GetGlobalOption("filemanager.compressparent") and not delete_y then
		goto_parent_dir()
		return
	end
	if delete_y then
		local second_table = {}
		for i = 1, #scanlist do
			if i == y then
				for x = i + 1, #scanlist do
					if scanlist[x].owner > y then
						scanlist[x]:decrease_owner(1)
					end
				end
			else
				second_table[#second_table + 1] = scanlist[i]
			end
		end
		scanlist = second_table
	end
	if tree_view:GetView().Width > (30 + highest_visible_indent) then
        tree_view:ResizePane(30 + highest_visible_indent)
	end
	refresh_and_select()
end
function prompt_delete_at_cursor()
	local y = get_safe_y()
	if y == 0 or scanlist_is_empty() then
		micro.InfoBar():Error("You can't delete that")
		return
	end
    micro.InfoBar():YNPrompt("Do you want to delete the " .. (scanlist[y].dirmsg ~= "" and "dir" or "file") .. ' "' .. scanlist[y].abspath .. '"? ', function(yes, canceled)
        if yes and not canceled then
            local go_os = import("os")
            local remove_log = go_os.RemoveAll(scanlist[y].abspath)
            if remove_log == nil then
                micro.InfoBar():Message("Filemanager deleted: ", scanlist[y].abspath)
                compress_target(get_safe_y(), true)
            else
                micro.InfoBar():Error("Failed deleting file/dir: ", remove_log)
            end
        else
            micro.InfoBar():Message("Nothing was deleted")
        end
    end)
end
local function update_current_dir(path)
	highest_visible_indent = 0
	tree_view:ResizePane(30)
	current_dir = path
	local scan_results = get_scanlist(path, 0, 0)
	if scan_results ~= nil then
		scanlist = scan_results
	else
		scanlist = {}
	end
	refresh_view()
	move_cursor_top()
end
local function go_back_dir()
	local one_back_dir = filepath.Dir(current_dir)
	if one_back_dir ~= current_dir then
		update_current_dir(one_back_dir)
	end
end
local function try_open_at_y(y)
	if y == 2 then
		go_back_dir()
	elseif y > 2 and not scanlist_is_empty() then
		y = y - 2
		if scanlist[y].dirmsg ~= "" then
			update_current_dir(scanlist[y].abspath)
		else
			micro.InfoBar():Message("Filemanager opened ", scanlist[y].abspath)
			micro.CurPane():VSplitIndex(buffer.NewBufferFromFile(scanlist[y].abspath), true)
		end
	else
		micro.InfoBar():Error("Can't open that")
	end
end
local function uncompress_target(y)
	if y == 0 or scanlist_is_empty() then
		return
	end
	if scanlist[y].dirmsg == "+" then
		local scan_results = get_scanlist(scanlist[y].abspath, y, scanlist[y].indent + 1)
		if scan_results ~= nil then
			local new_table = {}
			for i = 1, #scanlist do
				new_table[#new_table + 1] = scanlist[i]
				if i == y then
					for x = 1, #scan_results do
						new_table[#new_table + 1] = scan_results[x]
					end
					for inner_i = y + 1, #scanlist do
						if scanlist[inner_i].owner > y then
							scanlist[inner_i]:increase_owner(#scan_results)
						end
					end
				end
			end
			scanlist = new_table
		end
		scanlist[y].dirmsg = "-"
		if scan_results ~= nil then
			if scanlist[y].indent > highest_visible_indent and #scan_results >= 1 then
				highest_visible_indent = scanlist[y].indent
				tree_view:ResizePane(tree_view:GetView().Width + scanlist[y].indent)
			end
		end
		refresh_and_select()
	end
end
local function path_exists(path)
	local go_os = import("os")
	local file_stat, stat_err = go_os.Stat(path)
	if stat_err ~= nil then
		return go_os.IsExist(stat_err)
	elseif file_stat ~= nil then
		return true
	end
	return false
end
function rename_at_cursor(bp, args)
	if micro.CurPane() ~= tree_view then
		micro.InfoBar():Message("Rename only works with the cursor in the tree!")
		return
	end
	if #args < 1 then
		micro.InfoBar():Error('When using "rename" you need to input a new name')
		return
	end
	local new_name = args[1]
	local y = get_safe_y()
	if y == 0 then
		micro.InfoBar():Message("You can't rename that!")
		return
	end
	local old_path = scanlist[y].abspath
	local new_path = dirname_and_join(old_path, new_name)
	local golib_os = import("os")
	local log_out = golib_os.Rename(old_path, new_path)
	if log_out ~= nil then
		micro.Log("Rename log: ", log_out)
	end
	if not path_exists(new_path) then
		micro.InfoBar():Error("Path doesn't exist after rename!")
		return
	end
	scanlist[y].abspath = new_path
	refresh_and_select()
end
local function create_filedir(filedir_name, make_dir)
	if micro.CurPane() ~= tree_view then
		micro.InfoBar():Message("You can't create a file/dir if your cursor isn't in the tree!")
		return
	end
	if filedir_name == nil then
		micro.InfoBar():Error('You need to input a name when using "touch" or "mkdir"!')
		return
	end
	local y = get_safe_y()
	local filedir_path
	local scanlist_empty = scanlist_is_empty()
	if not scanlist_empty and y ~= 0 then
		if scanlist[y].dirmsg ~= "" then
			filedir_path = filepath.Join(scanlist[y].abspath, filedir_name)
		else
			filedir_path = dirname_and_join(scanlist[y].abspath, filedir_name)
		end
	else
		filedir_path = filepath.Join(current_dir, filedir_name)
	end
	if path_exists(filedir_path) then
		micro.InfoBar():Error("You can't create a file/dir with a pre-existing name")
		return
	end
	local golib_os = import("os")
	if make_dir then
		golib_os.Mkdir(filedir_path, golib_os.ModePerm)
		micro.Log("Filemanager created directory: " .. filedir_path)
	else
		golib_os.Create(filedir_path)
		micro.Log("Filemanager created file: " .. filedir_path)
	end
	if not path_exists(filedir_path) then
		micro.InfoBar():Error("The file/dir creation failed")
		return
	end
	local new_filedir = new_listobj(filedir_path, (make_dir and "+" or ""), 0, 0)
	local last_y
	if not scanlist_empty and y ~= 0 then
		last_y = tree_view.Cursor.Loc.Y + 1
		if scanlist[y].dirmsg == "+" then
			return
		elseif scanlist[y].dirmsg == "-" then
			new_filedir.owner = y
			new_filedir.indent = scanlist[y].indent + 1
		else
			new_filedir.owner = scanlist[y].owner
			new_filedir.indent = scanlist[y].indent
		end
		local new_table = {}
		for i = 1, #scanlist do
			new_table[#new_table + 1] = scanlist[i]
			if i == y then
				new_table[#new_table + 1] = new_filedir
				for inner_i = y + 1, #scanlist do
					if scanlist[inner_i].owner > y then
						scanlist[inner_i]:increase_owner(1)
					end
				end
			end
		end
		scanlist = new_table
	else
		scanlist[#scanlist + 1] = new_filedir
		last_y = #scanlist + tree_view.Cursor.Loc.Y
	end
	refresh_view()
	select_line(last_y)
end
function new_file(bp, args)
	if #args < 1 then
		micro.InfoBar():Error('When using "touch" you need to input a file name')
		return
	end
	local file_name = args[1]
	create_filedir(file_name, false)
end
function new_dir(bp, args)
	if #args < 1 then
		micro.InfoBar():Error('When using "mkdir" you need to input a dir name')
		return
	end
	local dir_name = args[1]
	create_filedir(dir_name, true)
end
local function open_tree()
	micro.CurPane():VSplitIndex(buffer.NewBuffer("", "filemanager"), false)
	tree_view = micro.CurPane()
    tree_view:ResizePane(30)
    tree_view.Buf.Type.Scratch = true
    tree_view.Buf.Type.Readonly = true
    tree_view.Buf:SetOptionNative("softwrap", true)
    tree_view.Buf:SetOptionNative("ruler", false)
    tree_view.Buf:SetOptionNative("autosave", false)
    tree_view.Buf:SetOptionNative("statusformatr", "")
    tree_view.Buf:SetOptionNative("statusformatl", "filemanager")
    tree_view.Buf:SetOptionNative("scrollbar", false)
	update_current_dir(os.Getwd())
end
local function close_tree()
	if tree_view ~= nil then
		tree_view:Quit()
		tree_view = nil
		clear_messenger()
	end
end
function toggle_tree()
	if tree_view == nil then
		open_tree()
	else
		close_tree()
	end
end
function uncompress_at_cursor()
	if micro.CurPane() == tree_view then
		uncompress_target(get_safe_y())
	end
end
function compress_at_cursor()
	if micro.CurPane() == tree_view then
		compress_target(get_safe_y(), false)
	end
end
function goto_prev_dir()
	if micro.CurPane() ~= tree_view or scanlist_is_empty() then
		return
	end
	local cur_y = get_safe_y()
	if cur_y ~= 0 then
		local move_count = 0
		for i = cur_y - 1, 1, -1 do
			move_count = move_count + 1
			if scanlist[i].dirmsg ~= "" then
				tree_view.Cursor:UpN(move_count)
				select_line()
				break
			end
		end
	end
end
function goto_next_dir()
	if micro.CurPane() ~= tree_view or scanlist_is_empty() then
		return
	end
	local cur_y = get_safe_y()
	local move_count = 0
	if cur_y == 0 then
		cur_y = 1
		move_count = 1
	end
	if cur_y < #scanlist then
		for i = cur_y + 1, #scanlist do
			move_count = move_count + 1
			if scanlist[i].dirmsg ~= "" then
				tree_view.Cursor:DownN(move_count)
				select_line()
				break
			end
		end
	end
end
function goto_parent_dir()
	if micro.CurPane() ~= tree_view or scanlist_is_empty() then
		return
	end
	local cur_y = get_safe_y()
	if cur_y > 0 then
		tree_view.Cursor:UpN(cur_y - scanlist[cur_y].owner)
		select_line()
	end
end
function try_open_at_cursor()
	if micro.CurPane() ~= tree_view or scanlist_is_empty() then
		return
	end
	try_open_at_y(tree_view.Cursor.Loc.Y)
end
local function false_if_tree(view)
	if view == tree_view then
		return false
	end
end
local function selectline_if_tree(view)
	if view == tree_view then
		select_line()
	end
end
local function aftermove_if_tree(view)
	if view == tree_view then
		if tree_view.Cursor.Loc.Y < 2 then
			tree_view.Cursor:DownN(2 - tree_view.Cursor.Loc.Y)
		end
		select_line()
	end
end
local function clearselection_if_tree(view)
	if view == tree_view then
		tree_view.Cursor:ResetSelection()
	end
end
function preQuit(view)
	if view == tree_view then
		close_tree()
		return false
	end
end
function preQuitAll(view)
	close_tree()
end
function preCursorDown(view)
	if view == tree_view then
		tree_view.Cursor:Down()
		select_line()
		return false
	end
end
function onCursorUp(view)
	selectline_if_tree(view)
end
function preParagraphPrevious(view)
	if view == tree_view then
		goto_prev_dir()
		return false
	end
end
function preParagraphNext(view)
	if view == tree_view then
		goto_next_dir()
		return false
	end
end
function onCursorPageUp(view)
	aftermove_if_tree(view)
end
function onCursorStart(view)
	aftermove_if_tree(view)
end
function onCursorPageDown(view)
	selectline_if_tree(view)
end
function onCursorEnd(view)
	selectline_if_tree(view)
end
function onNextSplit(view)
	selectline_if_tree(view)
end
function onPreviousSplit(view)
	selectline_if_tree(view)
end
function preMousePress(view, event)
    if view == tree_view and event ~= nil then
        if type(event) == "table" and event.Position ~= nil then
            local x, y = event:Position()
            local new_x, new_y = tree_view:GetMouseClickLocation(x, y)
            if new_y >= 2 then
                try_open_at_y(new_y)
            end
            return false
        else
            messenger:Error("Invalid mouse event received")
        end
    end
    return true
end
function onMouseScroll(view, event)
    if view == tree_view then
        local scroll_amount = event.ScrollAmount
        local current_line = tree_view.Buf.Cursor.Loc.Y
        if scroll_amount > 0 then
            tree_view.Buf.Cursor:UpN(scroll_amount)
        elseif scroll_amount < 0 then
            tree_view.Buf.Cursor:DownN(-scroll_amount)
        end
                select_line()
        return false
    end
end
function preCursorUp(view)
	if view == tree_view then
		if tree_view.Cursor.Loc.Y == 2 then
			return false
		end
	end
end
function preCursorLeft(view)
	if view == tree_view then
		compress_target(get_safe_y(), false)
		return false
	end
end
function preCursorRight(view)
    if view == tree_view then
        local y = get_safe_y()
        if y > 0 then
            if scanlist[y].dirmsg == "" then
                try_open_at_y(tree_view.Cursor.Loc.Y)
            else
                uncompress_target(y)
            end
        end
        return false
    end
end
local tab_pressed = false
function preIndentSelection(view)
	if view == tree_view then
		tab_pressed = true
		try_open_at_y(tree_view.Cursor.Loc.Y)
		return false
	end
end
function preInsertTab(view)
	if tab_pressed then
		tab_pressed = false
		return false
	end
end
function preInsertNewline(view)
    if view == tree_view then
        return false
    end
    return true
end
function onJumpLine(view)
	aftermove_if_tree(view)
end
function preSelectUp(view)
	if view == tree_view then
		goto_parent_dir()
		return false
	end
end
function preFind(view)
	clearselection_if_tree(view)
end
function onFind(view)
	selectline_if_tree(view)
end
function onFindNext(view)
	selectline_if_tree(view)
end
function onFindPrevious(view)
	selectline_if_tree(view)
end
local precmd_dir
function preCommandMode(view)
	precmd_dir = os.Getwd()
end
function onCommandMode(view)
	local new_dir = os.Getwd()
	if tree_view ~= nil and new_dir ~= precmd_dir and new_dir ~= current_dir then
		update_current_dir(new_dir)
	end
end
function preStartOfLine(view)
	return false_if_tree(view)
end
function preStartOfText(view)
    return false_if_tree(view)
end
function preEndOfLine(view)
	return false_if_tree(view)
end
function preMoveLinesDown(view)
	return false_if_tree(view)
end
function preMoveLinesUp(view)
	return false_if_tree(view)
end
function preWordRight(view)
	return false_if_tree(view)
end
function preWordLeft(view)
	return false_if_tree(view)
end
function preSelectDown(view)
	return false_if_tree(view)
end
function preSelectLeft(view)
	return false_if_tree(view)
end
function preSelectRight(view)
	return false_if_tree(view)
end
function preSelectWordRight(view)
	return false_if_tree(view)
end
function preSelectWordLeft(view)
	return false_if_tree(view)
end
function preSelectToStartOfLine(view)
	return false_if_tree(view)
end
function preSelectToStartOfText(view)
    return false_if_tree(view)
end
function preSelectToEndOfLine(view)
	return false_if_tree(view)
end
function preSelectToStart(view)
	return false_if_tree(view)
end
function preSelectToEnd(view)
	return false_if_tree(view)
end
function preDeleteWordLeft(view)
	return false_if_tree(view)
end
function preDeleteWordRight(view)
	return false_if_tree(view)
end
function preOutdentSelection(view)
	return false_if_tree(view)
end
function preOutdentLine(view)
	return false_if_tree(view)
end
function preSave(view)
	return false_if_tree(view)
end
function preCut(view)
	return false_if_tree(view)
end
function preCutLine(view)
	return false_if_tree(view)
end
function preDuplicateLine(view)
	return false_if_tree(view)
end
function prePaste(view)
	return false_if_tree(view)
end
function prePastePrimary(view)
	return false_if_tree(view)
end
function preMouseMultiCursor(view)
	return false_if_tree(view)
end
function preSpawnMultiCursor(view)
	return false_if_tree(view)
end
function preSelectAll(view)
	return false_if_tree(view)
end
function init()
    config.RegisterCommonOption("filemanager", "showdotfiles", true)
    config.RegisterCommonOption("filemanager", "showignored", true)
    config.RegisterCommonOption("filemanager", "compressparent", true)
    config.RegisterCommonOption("filemanager", "foldersfirst", true)
    config.RegisterCommonOption("filemanager", "openonstart", true)
    config.MakeCommand("tree", toggle_tree, config.NoComplete)
    config.MakeCommand("rename", rename_at_cursor, config.NoComplete)
    config.MakeCommand("touch", new_file, config.NoComplete)
    config.MakeCommand("mkdir", new_dir, config.NoComplete)
    config.MakeCommand("rm", prompt_delete_at_cursor, config.NoComplete)
    config.AddRuntimeFile("filemanager", config.RTSyntax, "filemanager.yaml")
    if config.GetGlobalOption("filemanager.openonstart") then
        if tree_view == nil then
            open_tree()
            micro.CurPane():NextSplit()
        else
            micro.Log(
                "Warning: filemanager.openonstart was enabled, but somehow the tree was already open so the option was ignored."
            )
        end
    end
end