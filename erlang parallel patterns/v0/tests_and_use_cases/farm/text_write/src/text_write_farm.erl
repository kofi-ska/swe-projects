-module(text_write_farm).
-export([file_write_worker/2, run/1]).


% Function to write text to a file
file_write_worker(Filename, Times) ->
    Text = "Hello, World!",
    case file:open(Filename, [write, append]) of
        {ok, File} ->
            write_text(File, Text, Times),
            file:close(File);
        {error, Reason} ->
            io:format("Worker failed to open file: ~p~n", [Reason])
    end.

% Helper function to write text N times
write_text(_, _, 0) ->
    ok;
write_text(File, Text, N) when N > 0 ->
    io:format(File, "~s~n", [Text]),
    write_text(File, Text, N - 1).

% Run function to execute the file writing in parallel
run(Times) ->
    Filename = "output_farm.txt",
    NumWorkers = erlang:system_info(schedulers_online), % Adjust the number of workers as needed

    % Call farm_work to perform the parallel file writing
    case farm_parallel_pattern:farm_work(Times, NumWorkers, fun file_write_worker/2, Filename) of
        ok ->
            io:format("File written successfully in parallel.~n");
        {error, Reason} ->
            io:format("Failed to write to file: ~p~n", [Reason])
    end.