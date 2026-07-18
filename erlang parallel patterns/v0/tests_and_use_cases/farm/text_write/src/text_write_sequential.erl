-module(text_write_sequential).
-export([write_to_file/3, create_or_overwrite_file/1, run/1]).

% Function to create or overwrite the file
create_or_overwrite_file(Filename) ->
    % Attempt to open the file in write mode (creates or overwrites)
    case file:open(Filename, [write]) of
        {ok, File} ->
            % Successfully opened the file
            file:close(File),
            ok;
        {error, Reason} ->
            % Return an error if unable to open the file
            {error, Reason}
    end.

% Function to write text to a file n times
write_to_file(Filename, Text, N) ->
    case file:open(Filename, [write]) of
        {ok, File} ->
            % Start writing text
            write_text(File, Text, N),
            % Close the file after writing
            file:close(File),
            ok;
        {error, Reason} ->
            % Return an error if unable to open the file
            {error, Reason}
    end.

% Helper function to write text N times
write_text(_, _, 0) ->
    ok;
write_text(File, Text, N) when N > 0 ->
    io:format(File, "~s~n", [Text]),
    write_text(File, Text, N - 1).

% Run function to take filename, text, and number of times
run(Times) ->
    Filename = "output.txt",
    Text = "Hello, World!",
    
    % Create or overwrite the file
    case create_or_overwrite_file(Filename) of
        ok ->
            % Write the text to the file N times
            case write_to_file(Filename, Text, Times) of
                ok ->
                    io:format("File written successfully.~n");
                {error, Reason} ->
                    io:format("Failed to write to file: ~p~n", [Reason])
            end;
        {error, Reason} ->
            io:format("Failed to create or overwrite file: ~p~n", [Reason])
    end.

