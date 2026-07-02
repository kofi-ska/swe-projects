-module(log_analysis_pipeline).
-export([run/0, analyze_log/1]).

run() ->
    analyze_log("log_file.txt").

analyze_log(Filename) ->
    % Define the stages for the pipeline
    Stages = [
        fun read_file/1,
        fun parse_logs/1,
        fun filter_logs/1,
        fun aggregate_data/1,
        fun generate_alerts/1,
        fun format_output/1,
        fun write_output/1
    ],

    % Run the pipeline with the defined stages
    PipelineResult = pipeline_parallel_pattern:run_pipeline(Stages, Filename),

    % Handle the result
    case PipelineResult of
        {ok, {ok, FileName}} when is_list(FileName) ->
            io:format("Log analysis complete. Output written to ~s~n", [FileName]);
        {ok, {error, Reason}} ->
            io:format("Log analysis failed: ~p~n", [Reason]);
        {error, Reason} ->
            io:format("Pipeline failed: ~p~n", [Reason])
    end.

%% Define the functions for log analysis

% Function to read a file
read_file(Filename) ->
    {ok, Data} = file:read_file(Filename),
    Lines = binary:split(Data, <<"\n">>, [global, trim_all]),
    lists:sublist(Lines, 2, length(Lines) - 1). % Skip header line

% Function to parse log entries into structured data
parse_logs(Lines) ->
    [parse_line(Line) || Line <- Lines].

parse_line(Line) ->
    [Date, Temp, Humidity, Light, Time, RecordedBy] = binary:split(Line, <<",">>, [global]),
    #{date => binary_to_list(Date),
      temp => binary_to_integer(Temp),
      humidity => binary_to_integer(Humidity),
      light_intensity => binary_to_integer(Light),
      time => binary_to_list(Time),
      recorded_by => binary_to_list(RecordedBy)}.

% Function to filter logs based on criteria
filter_logs(Logs) ->
    TempThreshold = 30,
    HumidityThreshold = 70,
    [Log || Log = #{temp := Temp, humidity := Humidity} <- Logs,
            Temp >= TempThreshold, Humidity >= HumidityThreshold].

% Function to aggregate data from logs
aggregate_data(Logs) ->
    TempCount = length([L || #{temp := Temp} = L <- Logs, Temp >= 30]),
    HumidityCount = length([L || #{humidity := Humidity} = L <- Logs, Humidity >= 70]),
    #{temp_count => TempCount, humidity_count => HumidityCount, logs => Logs}.

% Function to generate alerts based on aggregated data
generate_alerts(#{temp_count := TempCount, humidity_count := HumidityCount} = Data) when TempCount > 10 orelse HumidityCount > 10 ->
    Data#{alerts => [high_temp_or_humidity]};
generate_alerts(Data) ->
    Data#{alerts => []}.

% Function to format data into a string
format_output(#{temp_count := TempCount, humidity_count := HumidityCount, logs := Logs, alerts := Alerts}) ->
    LogLines = [format_log(Log) || Log <- Logs],
    AlertsLine = format_alerts(Alerts),
    Summary = io_lib:format("Summary:\n  Temperature Count: ~p\n  Humidity Count: ~p\n", [TempCount, HumidityCount]),
    Result = Summary ++ AlertsLine ++ LogLines,
    io_lib:format("~s", [Result]).

% Format individual log entry
format_log(#{date := Date, temp := Temp, humidity := Humidity, light_intensity := Light, time := Time, recorded_by := RecordedBy}) ->
    io_lib:format("Date: ~s, Temp: ~p, Humidity: ~p, Light: ~p, Time: ~s, Recorded By: ~s\n", [Date, Temp, Humidity, Light, Time, RecordedBy]).

% Format alerts
format_alerts([]) ->
    "No alerts.\n";
format_alerts(Alerts) ->
    AlertsLine = "Alerts:\n",
    AlertsLine ++ io_lib:format("  ~p\n", [Alerts]).

% Function to write formatted data to a file
write_output(FormattedData) ->
    case file:write_file("processed_log_file.txt", FormattedData) of
        ok -> {ok, "processed_log_file.txt"};
        {error, Reason} -> {error, Reason}
    end.
