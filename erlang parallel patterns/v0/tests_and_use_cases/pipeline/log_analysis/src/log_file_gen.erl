-module(log_file_gen).
-export([create/2]).

% API function to generate log data and write to a file
create(Filename, NumRecords) ->
    Data = generate_log_data(NumRecords),
    write_to_file(Filename, Data),
    Filename.

% Function to generate random log data
generate_log_data(NumRecords) ->
    Headers = "date,temp,humidity,light_intensity,time,recorded_by\n",
    Records = generate_records(NumRecords),
    [Headers | Records].

generate_records(NumRecords) ->
    lists:map(fun(_) -> generate_record() end, lists:seq(1, NumRecords)).

generate_record() ->
    Date = random_date(),
    Temp = random_value(-2, 40),
    Humidity = random_value(2, 100),
    Light = random_value(23, 300),
    Time = random_time(),
    Sensor = random_sensor(),
    io_lib:format("~s,~p,~p,~p,~s,~s\n", [Date, Temp, Humidity, Light, Time, Sensor]).

random_date() ->
    Date = calendar:date_to_gregorian_days({2024, 1, 1}) + rand:uniform(365),
    {Y, M, D} = calendar:gregorian_days_to_date(Date),
    io_lib:format("~4..0w-~2..0w-~2..0w", [Y, M, D]).

random_value(Min, Max) ->
    Min + rand:uniform(Max - Min).

random_time() ->
    Hour = rand:uniform(23),
    Minute = rand:uniform(59),
    Second = rand:uniform(59),
    io_lib:format("~2..0w:~2..0w:~2..0w", [Hour, Minute, Second]).

random_sensor() ->
    Sensors = ["Sensor1", "Sensor2", "Sensor3", "Sensor4"],
    lists:nth(rand:uniform(length(Sensors)), Sensors).

% Function to write data to a file
write_to_file(Filename, Data) ->
    case file:open(Filename, [write]) of
        {ok, File} ->
            lists:foreach(fun(Line) -> file:write(File, Line) end, Data),
            file:close(File),
            ok;
        {error, Reason} ->
            {error, Reason}
    end.
