-module(log_file_create).
-export([create_log_file/1]).


create_log_file(NumOfLines) ->
    log_file_gen:create("log_file.txt", NumOfLines).