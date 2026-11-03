# Import a Destination by its numeric id. Credentials are never read back: add them to config
# before the next apply. The first plan also lists settings Hevo has that config does not set;
# those are not sent, and the plan settles after one apply.
terraform import hevo_destination.snowflake 7
